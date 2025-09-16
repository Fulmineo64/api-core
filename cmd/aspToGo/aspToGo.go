package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

/*
# Prompt

Traduci il seguente codice ASP in Go templ, seguendo le seguenti indicazioni:
- rimpiazza <div class="row"> con @Row, se vi sono altre classi oltre a "row" esempio <div class="row row-spaced align-items-center"> diventa @Row(Class("row-spaced align-items-center"){...}
- rimpiazza i <div class="col..."> con @Col riportando le classi senza "col-" davanti esempio <div class="col-6 col-md-3 col-xxl"> con @Col("6 md-3 xxl"){...}
- rimpiazza <label> e <input> con @FieldInput(label, field string, attrs ...Attrs) dove gli input hanno l'attributo ng-model che inizia con $ctrl.model, metti il contenuto della label nel primo parametro, il campo (ovvero la parte dopo $ctrl.model. in ng-model) nel secondo parametro e i restanti attributi in Attrs{} nel 3° parametro che puoi usare come una map[string]any
- rimpiazza lang("stringa") con Lang(c, "stringa")
- rimpiazza <% if ... then %> e <% end if %> con if ... { ... }
- rimpiazza Permissions.has(...) e Permissions.hasOne(...) con s.Has(...) e s.HasOne(...)
- rimpiazza Application("FLAG_...") con app.Flags.Has("...") senza FLAG_ davanti, se Application("...") non inizia con FLAG_ rimpiazzala invece con la map[string]string app.Properties["..."]
- al posto di <%= %> per scrivere i valori usa { }
*/

// funzione che processa una stringa e stampa sia versione parsata sia raw
func process(input string) {
	z := html.NewTokenizer(strings.NewReader(input))

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			// fine input
			return

		case html.StartTagToken, html.SelfClosingTagToken:
			tagName, hasAttr := z.TagName()
			fmt.Println("TAG:", string(tagName))
			fmt.Println(" RAW:", string(z.Raw())) // il tag così com’è nell’input

			for hasAttr {
				key, val, more := z.TagAttr()
				fmt.Printf("  ATTR: %s = %q\n", key, val)
				hasAttr = more
			}

		case html.TextToken:
			text := string(z.Text())
			raw := string(z.Raw())
			if strings.Contains(raw, "<%") {
				fmt.Println("ASP BLOCK:", raw) // blocco ASP originale
			} else if strings.TrimSpace(text) != "" {
				fmt.Println("TEXT:", text)
			}

		case html.EndTagToken:
			tagName, _ := z.TagName()
			fmt.Println("ENDTAG:", string(tagName), "RAW:", string(z.Raw()))

		case html.CommentToken:
			fmt.Println("COMMENT:", string(z.Raw()))

		case html.DoctypeToken:
			fmt.Println("DOCTYPE:", string(z.Raw()))
		}
	}
}

func main() {
	inPath := flag.String("in", "", "input file (default stdin)")
	outPath := flag.String("out", "", "output file (default stdout)")
	flag.Parse()

	pageName := "Parsed"

	// sorgente input
	var r io.Reader
	if *inPath == "" {
		fmt.Println("🔧 Parsing da ASP a Go di stdin")
		r = os.Stdin
	} else {
		fmt.Println("🔧 Parsing da ASP a Go di " + *inPath)
		f, err := os.Open(*inPath)
		if err != nil {
			exitErr("errore in apertura input", err)
		}
		defer f.Close()

		_, file := path.Split(*inPath)
		pageName = strings.TrimSuffix(file, ".asp")
		parts := strings.Split(pageName, "_")
		for i := 1; i < len(parts); i++ {
			if len(parts[i]) > 0 {
				runes := []rune(parts[i])
				runes[0] = unicode.ToUpper(runes[0])
				parts[i] = string(runes)
			}
		}
		pageName = strings.Join(parts, "")
		r = f
	}

	inBytes, err := io.ReadAll(r)
	if err != nil {
		exitErr("errore in lettura", err)
	}

	re := regexp.MustCompile(`(<%=\s*)(.*?)(\s*%>)`)
	inStr := re.ReplaceAllStringFunc(string(inBytes), func(m string) string {
		parts := re.FindStringSubmatch(m)
		return parts[1] + strings.ReplaceAll(parts[2], `"`, "`") + parts[3]
	})

	doc, err := html.Parse(bytes.NewBufferString(inStr))
	if err != nil {
		exitErr("errore in parsing HTML", err)
	}

	var out string
	out = "package pages\n\n"
	out += "templ " + pageName + "() {\n"

	t := NewTransformer()
	t.Transform(doc)
	out += t.String()

	if *outPath == "" {
		fmt.Print(out)
	} else {
		if err := os.WriteFile(*outPath, []byte(out), 0644); err != nil {
			exitErr("errore in scrittura", err)
		}
		fmt.Println("✅ output in " + *outPath)
	}

}

func exitErr(msg string, err error) {
	fmt.Fprintln(os.Stderr, "❌ "+msg+":", err)
	os.Exit(2)
}

func NewTransformer() *Transformer {
	return &Transformer{
		reLang: regexp.MustCompile(`<%=?\s*lang\((?:"|')(.*?)(?:"|')\)\s*%>`),
		reApp:  regexp.MustCompile(`Application\((?:"|')(.*?)(?:"|')\)`),
	}
}

type Transformer struct {
	reLang      *regexp.Regexp
	reApp       *regexp.Regexp
	indentLevel int
	b           strings.Builder
}

func (t *Transformer) add(str string) *Transformer {
	t.b.WriteString(str)
	return t
}

func (t *Transformer) indent() *Transformer {
	t.b.WriteString(strings.Repeat("\t", t.indentLevel))
	return t

}

func (t *Transformer) ln() *Transformer {
	t.add("\n")
	return t
}

func (t *Transformer) Transform(n *html.Node) {
	if n.Type == html.DocumentNode {
		t.transformChildren(n)
	} else if n.Type == html.CommentNode {
		t.indent().add("<!--" + n.Data + "-->").ln()
	} else if n.Type == html.ElementNode {
		class := ""
		for _, a := range n.Attr {
			if a.Key == "class" {
				class = t.parseAttr(a.Val)
				break
			}
		}

		switch {
		case n.Data == "div" && strings.Contains(class, "row"):
			classes := strings.Fields(class)
			extra := []string{}
			for _, c := range classes {
				if c != "row" {
					extra = append(extra, c)
				}
			}
			if len(extra) == len(classes) {
				t.renderPlain(n)
				return
			}
			t.indent().add(`@Row(`)
			if len(extra) > 0 {
				t.add(`Class("` + strings.Join(extra, " ") + `")`)
			}
			t.add(`) {`).ln()
			t.transformChildren(n)
			t.indent().add("}")
			return
		case n.Data == "div" && strings.Contains(class, "col-"):
			colSizes := []string{}
			extra := []string{}
			classes := strings.Fields(class)
			for _, class := range classes {
				if strings.HasPrefix(class, "col-") {
					colSizes = append(colSizes, strings.TrimPrefix(class, "col-"))
				} else {
					extra = append(extra, class)
				}
			}
			t.indent().add(`@Col("` + strings.Join(colSizes, " ") + `"`)
			if len(extra) > 0 {
				t.add(`, Class("` + strings.Join(extra, " ") + `")`)
			}
			t.add(`) {`).ln()
			t.transformChildren(n)
			t.indent().add("}").ln()
			return
		case n.Data == "label":
			// Expects a text content and an input inside the label
			var text string
			var input *html.Node
			var count int
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.TextNode {
					text = c.Data
				} else if c.Type == html.ElementNode && c.Data == "input" {
					input = c
				}
				count++
			}
			if count == 2 && text != "" && input != nil {
				var field string
				var attrs []html.Attribute
				for _, a := range input.Attr {
					if a.Key == "ng-model" {
						field = strings.TrimPrefix(t.parseAttr(a.Val), "$ctrl.model.")
					} else {
						attrs = append(attrs, a)
					}
				}
				if field == "" {
					t.renderPlain(input)
					return
				}
				t.indent().add(fmt.Sprintf(`@FieldInput(%s, "%s", %s)`, t.parseAttr(text), field, t.renderTemplAttrs(attrs)))
				return
			} else {
				t.renderPlain(n)
				return
			}
		}
		t.renderPlain(n)
		return
	}

	data := strings.TrimSpace(n.Data)
	if n.Type == html.TextNode && len(data) > 0 {
		if strings.HasPrefix(data, "<%") && !strings.HasPrefix(data, "<%=") {
			lines := strings.Split(data, "\n")
			for _, line := range lines {
				t.indent().add(line).ln()
			}
		} else {
			t.indent().add(t.parseAttr(data)).ln()
		}
	}
}

func (t *Transformer) String() string {
	return t.b.String()
}

func (t *Transformer) transformChildren(n *html.Node) {
	t.indentLevel++
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		t.Transform(c)
	}
	t.indentLevel--
}

func (t *Transformer) parseAttr(val string) string {
	val = strings.ReplaceAll(val, "<%=", "{")
	val = strings.ReplaceAll(val, "%>", "}")
	val = strings.ReplaceAll(val, "writeApplication(`FLAG_", "app.Flags.Has(`")
	val = strings.ReplaceAll(val, "Application(`FLAG_", "app.Flags.Has(`")
	val = regexp.MustCompile("Application\\(`(.*)`\\)").ReplaceAllString(val, "app.Properties[`$1`]")
	val = strings.ReplaceAll(val, "Application(`", "app.Properties[`")
	val = strings.ReplaceAll(val, "lang(", "Lang(c,")
	return val
}

func (t *Transformer) renderTemplAttrs(attrs []html.Attribute) string {
	if len(attrs) == 0 {
		return "Attrs{}"
	}
	var b strings.Builder
	b.WriteString("Attrs{\n")
	for _, a := range attrs {
		val := t.parseAttr(a.Val)
		if val == "" {
			fmt.Fprintf(&b, "\t%q: true,\n", a.Key)
		} else {
			fmt.Fprintf(&b, "\t%q: %q,\n", a.Key, val)
		}
	}
	b.WriteString("}")
	return b.String()
}

var voidElements = map[string]struct{}{
	"area":   {},
	"base":   {},
	"br":     {},
	"col":    {},
	"embed":  {},
	"hr":     {},
	"img":    {},
	"input":  {},
	"link":   {},
	"meta":   {},
	"param":  {},
	"source": {},
	"track":  {},
	"wbr":    {},
}

func (t *Transformer) renderPlain(n *html.Node) {
	t.indent().add("<" + n.Data)
	for _, a := range n.Attr {
		t.add(" " + a.Key + `="` + t.parseAttr(a.Val) + `"`)
	}
	t.add(">")
	if n.FirstChild != nil {
		t.ln()
		t.transformChildren(n)
	}
	if _, ok := voidElements[n.Data]; !ok {
		if n.FirstChild != nil {
			t.indent()
		}
		t.add("</" + n.Data + ">")
	}
	t.ln()
}
