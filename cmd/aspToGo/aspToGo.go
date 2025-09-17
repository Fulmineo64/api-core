package main

import (
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

	tokenizer := html.NewTokenizer(strings.NewReader(inStr))
	if err != nil {
		exitErr("errore in parsing HTML", err)
	}

	var out string
	out = "package pages\n\n"
	out += "templ " + pageName + "() {\n"

	out += NewTransformer().Transform(tokenizer)

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

func (t *Transformer) Transform(z *html.Tokenizer) string {
	var prev html.TokenType
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return t.b.String()
		case html.StartTagToken, html.SelfClosingTagToken:
			if prev == html.StartTagToken {
				t.ln()
			}
			tagName, hasAttr := z.TagName()
			tag := string(tagName)
			raw := string(z.Raw())

			var class string
			var attrs []html.Attribute
			for hasAttr {
				k, v, more := z.TagAttr()
				attrs = append(attrs, html.Attribute{Key: string(k), Val: string(v)})
				if string(k) == "class" {
					class = string(v)
				}
				hasAttr = more
			}

			switch {
			case tag == "div" && strings.Contains(class, "row"):
				classes := strings.Fields(class)
				extra := []string{}
				for _, c := range classes {
					if c != "row" {
						extra = append(extra, c)
					}
				}
				if len(extra) != len(classes) {
					t.indent().add(`@Row(`)
					if len(extra) > 0 {
						t.add(`Class("` + strings.Join(extra, " ") + `")`)
					}
					t.add(`) {`).ln()
					// t.indent().add("}")
				}
			case tag == "div" && strings.Contains(class, "col-"):
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
				// t.indent().add("}").ln()
			case tag == "label":
				// Expects a text content and an input inside the label
				// var text string
				// var input *html.Node
				// var count int
				// for c := n.FirstChild; c != nil; c = c.NextSibling {
				// 	if c.Type == html.TextNode {
				// 		text = c.Data
				// 	} else if c.Type == html.ElementNode && c.Data == "input" {
				// 		input = c
				// 	}
				// 	count++
				// }
				// if count == 2 && text != "" && input != nil {
				// 	var field string
				// 	var attrs []html.Attribute
				// 	for _, a := range input.Attr {
				// 		if a.Key == "ng-model" {
				// 			field = strings.TrimPrefix(t.parseAttr(a.Val), "$ctrl.model.")
				// 		} else {
				// 			attrs = append(attrs, a)
				// 		}
				// 	}
				// 	if field == "" {
				// 		t.renderPlain(input)
				// 		return
				// 	}
				// 	t.indent().add(fmt.Sprintf(`@FieldInput(%s, "%s", %s)`, t.parseAttr(text), field, t.renderTemplAttrs(attrs)))
				// 	return
				// } else {
				// 	t.renderPlain(n)
				// 	return
				// }
			default:
				if t.hasASPCodeTag(raw) {
					t.indent().add(raw)
				} else {
					t.indent().add("<" + tag + t.Attrs(attrs) + ">")
				}
			}
			if tt != html.SelfClosingTagToken {
				if _, ok := voidElements[tag]; !ok {
					t.indentLevel++
				}
			}

		case html.EndTagToken:
			raw := string(z.Raw())
			t.indentLevel--
			t.indent().add(raw).ln()

		case html.TextToken:
			if prev == html.StartTagToken {
				t.ln()
			}
			raw := strings.TrimSpace(string(z.Raw()))
			if strings.Contains(raw, "<%") {
				lines := strings.Split(raw, "\n")
				for _, line := range lines {
					t.indent().add(line).ln()
				}
			} else if raw != "" {
				t.indent().add(t.parseAttr(raw)).ln()
			}

		case html.CommentToken:
			if prev == html.StartTagToken {
				t.ln()
			}
			t.indent().add(string(z.Raw())).ln()
		}
		prev = tt
	}
}

func (t *Transformer) hasASPCodeTag(s string) bool {
	re := regexp.MustCompile(`<%`)
	matches := re.FindAllStringIndex(s, -1)
	for _, m := range matches {
		// controlla se subito dopo c'è un '='
		if m[1] < len(s) && s[m[1]] != '=' {
			return true
		}
	}
	return false
}

func (t *Transformer) parseAttr(val string) string {
	val = strings.ReplaceAll(val, "<%=", "{")
	val = strings.ReplaceAll(val, "%>", "}")
	val = strings.ReplaceAll(val, "writeApplication(`FLAG_", "app.Flags.Has(`")
	val = strings.ReplaceAll(val, "Application(`FLAG_", "app.Flags.Has(`")
	val = regexp.MustCompile("Application\\(`(.*)`\\)").ReplaceAllString(val, "app.Properties[`$1`]")
	val = strings.ReplaceAll(val, "Request(`", "c.Query(`")
	val = strings.ReplaceAll(val, "Application(`", "app.Properties[`")
	val = strings.ReplaceAll(val, "lang(", "Lang(c,")
	if !strings.HasPrefix(val, "{") && !strings.HasSuffix(val, "}") {
		val = `"` + val + `"`
	}
	return val
}

func (t *Transformer) Attrs(attrs []html.Attribute) string {
	var b strings.Builder
	for _, attr := range attrs {
		b.WriteString(" ")
		if attr.Val == "" {
			b.WriteString(attr.Key)
		} else {
			b.WriteString(attr.Key + "=" + t.parseAttr(attr.Val))
		}
	}
	return b.String()
}

func (t *Transformer) TemplAttrs(attrs []html.Attribute) string {
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
