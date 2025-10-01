package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

func main() {
	inPath := flag.String("in", "", "input file (obbligatorio)")
	outPath := flag.String("out", "", "output file (obbligatorio)")
	flag.Parse()

	if *inPath == "" || *outPath == "" {
		fmt.Println("❌ Devi specificare sia -in che -out")
		os.Exit(1)
	}

	// pageName ricavato dal file di input
	pageName := "Parsed"
	fmt.Println("🔧 Parsing da ASP a Go di " + *inPath)

	f, err := os.Open(*inPath)
	if err != nil {
		exitErr("errore in apertura input", err)
	}
	defer f.Close()

	_, file := path.Split(*inPath)
	extIndex := strings.LastIndex(file, ".")
	pageName = file[:extIndex]
	re := regexp.MustCompile(`[\.\-_]+`)
	parts := re.Split(pageName, -1)
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			runes := []rune(parts[i])
			runes[0] = unicode.ToUpper(runes[0])
			parts[i] = string(runes)
		}
	}
	pageName = strings.Join(parts, "")

	inBytes, err := io.ReadAll(f)
	if err != nil {
		exitErr("errore in lettura", err)
	}

	templBody := NewTransformer().Transform(string(inBytes))

	// prendi nome package dall'ultima cartella dell'outPath
	outDir := path.Dir(*outPath)
	_, pkg := path.Split(outDir)
	if pkg == "" {
		pkg = "main" // fallback se non riesce
	}

	var out string
	out = "package " + pkg + "\n\n"
	out += "import ("
	out += "\n\t\"api_core/request\""
	out += "\n\t. \"clima_api/views/html/common\""
	out += "\n\t\"github.com/gin-gonic/gin\""
	out += "\n\t\"clima_api/controllers\""

	if regexp.MustCompile(`{.*html\..*}`).MatchString(templBody) {
		out += "\n\t\"html\""
	}
	out += "\n)\n\n"
	out += "templ " + pageName + "() {\n"
	out += templBody
	out += "}"

	if err := os.WriteFile(*outPath, []byte(out), 0644); err != nil {
		exitErr("errore in scrittura", err)
	}
	fmt.Println("✅ output in " + *outPath)
}

func exitErr(msg string, err error) {
	fmt.Fprintln(os.Stderr, "❌ "+msg+":", err)
	os.Exit(2)
}

// Attrs

func parseAttr(val string) string {
	val = replaceKeywords(val)
	if !strings.HasPrefix(val, "{") && !strings.HasSuffix(val, "}") {
		val = `"` + val + `"`
	}
	return val
}

func templAttrs(attrs []html.Attribute) string {
	if len(attrs) == 0 {
		return ""
	}
	result := []string{}
	for _, a := range attrs {
		if a.Val == "" {
			result = append(result, "`"+a.Key+"`: true")
		} else {
			result = append(result, "`"+a.Key+"`: `"+replaceKeywords(a.Val)+"`")
		}
	}
	return "Attrs{" + strings.Join(result, ", ") + "}"
}

// Keywords

func replaceKeywords(val string) string {
	val = replaceASPKeywords(val)
	val = replaceAngularKeywords(val)
	return val
}

func replaceASPKeywords(val string) string {
	// if
	val = regexp.MustCompile(`(?i)<%\s*if\s+(.*)\sthen\s*%>(.*)<%\s*end if\s*%>`).ReplaceAllString(val, "if $1 { $2 }")
	val = regexp.MustCompile(`(?i)<%\s*if\s+(.*)\sthen\s*%>`).ReplaceAllString(val, "if $1 {")
	val = regexp.MustCompile(`(?i)<%\s*end\s+if\s*%>`).ReplaceAllString(val, "}")
	// operators
	val = regexp.MustCompile(`(?i) and `).ReplaceAllString(val, " && ")
	val = regexp.MustCompile(`(?i) or `).ReplaceAllString(val, " || ")
	val = regexp.MustCompile(`(?i) not `).ReplaceAllString(val, " !")
	// array
	val = regexp.MustCompile(`(?i)array\((.*?)\)`).ReplaceAllString(val, "$1")
	// functions and keywords
	val = regexp.MustCompile(`(?i)writeApplication\("FLAG_([\w_]*?)"\)`).ReplaceAllString(val, "app.Flags.Has(`$1`)")
	val = regexp.MustCompile(`(?i)Application\("FLAG_([\w_]*?)"\)`).ReplaceAllString(val, "app.Flags.Has(`$1`)")
	val = regexp.MustCompile(`(?i)writeApplication\("(.*?)"\)`).ReplaceAllString(val, "app.Properties[`$1`]")
	val = regexp.MustCompile(`(?i)Application\("(.*?)"\)`).ReplaceAllString(val, "app.Properties[`$1`]")
	val = regexp.MustCompile(`(?i)Server.HTMLEncode\(`).ReplaceAllString(val, "html.EscapeString(")
	val = regexp.MustCompile(`(?i)Session\("(.*?)"\)`).ReplaceAllString(val, "s.GetString(`$1`)")

	val = regexp.MustCompile(`(?i)Request\(`).ReplaceAllString(val, "c.Query(")
	val = regexp.MustCompile(`(?i)Permissions.has\(`).ReplaceAllString(val, "s.Has(")
	val = regexp.MustCompile(`lang\(`).ReplaceAllString(val, "Lang(c,")
	// tags
	val = strings.ReplaceAll(val, "<%=", "{")
	val = strings.ReplaceAll(val, "%>", "}")
	return val
}

func replaceAngularKeywords(val string) string {
	val = regexp.MustCompile(`{{(.*?)}}`).ReplaceAllString(val, "@W(`$1`)")
	return val
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
		replace: map[string][]string{},
	}
}

type Transformer struct {
	indentLevel int
	b           strings.Builder
	replace     map[string][]string
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

func (t *Transformer) Transform(in string) string {
	var mainSection bool
	var controller, tipo string
	// Sposta tutto il contenuto prima di div class container dentro di esso
	re := regexp.MustCompile(`(?sm)(.*)<!--#include file="header.asp"-->(.*?)(<div class="container.*?".*?ng-app.*?>)`)
	in = re.ReplaceAllString(in, "$1@Head()$3$2")
	// Rimuove gli import ASP già gestiti
	re = regexp.MustCompile(`\s*<!--\s*#include file=".*?(util\/Permissions.asp|secure.asp)"-->`)
	in = re.ReplaceAllString(in, "")
	// Individua se la pagina è un elenco o una scheda
	re = regexp.MustCompile(`\s*<script type="module" src="(.*?)"></script>`)
	in = re.ReplaceAllStringFunc(in, func(m string) string {
		parts := re.FindStringSubmatch(m)
		re = regexp.MustCompile(`js\/app-(.*)-(scheda|elenco)\.js.*`)
		controller = re.ReplaceAllString(parts[1], "$1")
		controller = strings.ToUpper(controller[0:1]) + controller[1:]
		if strings.Contains(parts[1], "-elenco.js") {
			tipo = "Elenco"
		} else {
			tipo = "Scheda"
		}
		return ""
	})
	// Trasforma gli script in import AngularApp
	scripts := []string{}
	re = regexp.MustCompile(`\s*<script (type="text\/javascript" |)src="[\.\/]*(.*?)(\?v=.*|)">\s*<\/script>`)
	in = re.ReplaceAllStringFunc(in, func(m string) string {
		parts := re.FindStringSubmatch(m)
		scripts = append(scripts, parts[2])
		return ""
	})
	// Rimpiazza " nei segmenti ASP inlinea con `
	re = regexp.MustCompile(`(\s*<\w+.*?)("*<%\s*.*\s*%>"*)((.*?)>)`)
	in = re.ReplaceAllStringFunc(in, func(m string) string {
		parts := re.FindStringSubmatch(m)
		if strings.HasPrefix(parts[2], `"`) && strings.HasSuffix(parts[2], `"`) {
			parts[2] = parts[2][1 : len(parts[2])-1]
		}
		reAsp := regexp.MustCompile(`<%\s*.*?\s*%>`)
		parts[2] = reAsp.ReplaceAllStringFunc(parts[2], func(m string) string {
			parts := reAsp.FindStringSubmatch(m)
			return strings.ReplaceAll(replaceKeywords(parts[0]), `"`, "`")
		})
		result := parts[1] + parts[2] + parts[3]
		return result
	})

	// Parsing start
	z := html.NewTokenizer(strings.NewReader(in))
	t.indentLevel++
	t.indent().add("{{").ln()
	t.indentLevel++
	t.indent().add("c := ctx.Value(GinKey).(*gin.Context)").ln()
	t.indent().add("s := request.Session(c)").ln()
	if controller != "" {
		t.indent().add("a := NewAngularApp(controllers." + controller + "{})").ln()
		for _, script := range scripts {
			t.indent().add(`a.Scripts.Add("` + script + `")`).ln()
		}
	}
	t.indentLevel--
	t.indent().add("}}").ln()
	var prev html.TokenType
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			return t.finalize(t.b.String())
		}
		raw := strings.TrimSpace(string(z.Raw()))
		switch tt {
		case html.StartTagToken, html.SelfClosingTagToken:
			if prev == html.StartTagToken {
				t.ln()
			}
			tagName, hasAttr := z.TagName()
			tag := string(tagName)

			attrs := map[string]string{}
			for hasAttr {
				k, v, more := z.TagAttr()
				attrs[string(k)] = string(v)
				hasAttr = more
			}

			switch {
			// Trasformazione di div class="container" e ng-app in @Scheda o Elenco
			case tag == "div" && strings.Contains(attrs["class"], "container") && attrs["ng-app"] != "":
				t.indent().add(`@` + tipo + `(a, "` + strings.TrimSpace(strings.ReplaceAll(attrs["class"], "container", "")) + `") {`)
				t.replaceNext("div", t.indentLevel, "}")
			// Rimuove la form base all'inizio della pagina
			case tag == "form" && strings.Contains(attrs["name"], "form1"):
				tt = html.SelfClosingTagToken
				t.replaceNext("form", t.indentLevel-1, "")
			// Rimuove la section base all'inizio della pagina
			case attrs["class"] == "section" && !mainSection:
				mainSection = true
				tt = html.SelfClosingTagToken
				t.replaceNext(tag, t.indentLevel-1, "")
			// Trasformazione di div in @Row
			case tag == "div" && strings.Contains(attrs["class"], "row"):
				classes := strings.Fields(attrs["class"])
				extra := []string{}
				for _, c := range classes {
					if c != "row" {
						extra = append(extra, c)
					}
				}
				if len(extra) != len(classes) {
					t.indent().add("@Row(")
					if len(extra) > 0 {
						t.add("Class(`" + strings.Join(extra, " ") + "`)")
					}
					t.add(`) {`)
					t.replaceNext("div", t.indentLevel, "}")
				}
			// Trasformazione di div in @Col
			case tag == "div" && strings.Contains(attrs["class"], "col-"):
				colSizes := []string{}
				extra := []string{}
				classes := strings.Fields(attrs["class"])
				for _, class := range classes {
					if strings.HasPrefix(class, "col-") {
						colSizes = append(colSizes, strings.TrimPrefix(class, "col-"))
					} else {
						extra = append(extra, class)
					}
				}
				t.indent().add("@Col(`" + strings.Join(colSizes, " ") + "`")
				if len(extra) > 0 {
					t.add(", Class(`" + strings.Join(extra, " ") + "`)")
				}
				t.add(`) {`)
				t.replaceNext("div", t.indentLevel, "}")
			default:
				aspBeginIndex := strings.Index(raw, "<%")
				for aspBeginIndex != -1 {
					aspEndIndex := strings.Index(raw, "%>") + 2
					// TODO: Parte obsoleta dopo initialize, da sistemare
					if aspBeginIndex >= 2 && raw[aspBeginIndex-2:aspBeginIndex] == `="` {
						raw = raw[:aspBeginIndex-1] + parseAttr(raw[aspBeginIndex:aspEndIndex]) + raw[aspEndIndex+1:]
					} else {
						raw = raw[:aspBeginIndex] + replaceKeywords(raw[aspBeginIndex:aspEndIndex]) + raw[aspEndIndex:]
					}
					newBeginIndex := strings.Index(raw, "<%")
					if newBeginIndex == aspBeginIndex {
						panic("Errore nel parsing di " + string(z.Raw()))
					}
					aspBeginIndex = newBeginIndex
				}
				t.indent().add(raw)
			}
			if tt != html.SelfClosingTagToken {
				if _, ok := voidElements[tag]; ok {
					t.ln()
					tt = html.SelfClosingTagToken
				} else {
					t.indentLevel++
				}
			}
		case html.EndTagToken:
			t.indentLevel--
			if t.indentLevel < 0 {
				t.indentLevel = 0
			}

			tagName, _ := z.TagName()
			key := fmt.Sprintf("%s:%v", string(tagName), t.indentLevel)
			if replacements, ok := t.replace[key]; ok && len(replacements) > 0 {
				replacement := replacements[0]
				if replacement == "" {
					t.indentLevel++
				} else {
					if prev != html.StartTagToken {
						t.indent()
					}
					t.add(replacement).ln()
				}
				t.replace[key] = replacements[1:]
			} else {
				if prev != html.StartTagToken {
					t.indent()
				}
				t.add(raw).ln()
			}
		case html.TextToken:
			if prev == html.StartTagToken {
				t.ln()
			}
			// Sostituzione keywords ASP e Angular
			if raw != "" {
				if strings.Contains(raw, "<%") {
					lines := strings.Split(raw, "\n")
					for _, line := range lines {
						if strings.Contains(line, "<%") && strings.Contains(line, "%>") {
							// Se la riga inizia e finisce con una tag ASP tento di convertirla automaticamente
							text := replaceKeywords(strings.TrimSpace(line))
							if !strings.Contains(text, "{") && strings.HasSuffix(text, "}") {
								t.indentLevel--
							}
							t.indent().add(text).ln()
							if strings.HasSuffix(text, "{") {
								t.indentLevel++
							}
						} else {
							t.add(line).ln()
						}
					}
				} else {
					t.indent().add(replaceAngularKeywords(raw)).ln()
				}
			}
		case html.CommentToken:
			if prev == html.StartTagToken {
				t.ln()
			}
			t.indent().add(raw).ln()
		}
		prev = tt
	}
}

func (t *Transformer) finalize(text string) string {
	textLabelRegex := `(?m)(\s*)<label class="form-control-label">\s*(.[^<>]+?)\s*<\/label>`
	// Rimpiazza le <label class="form-control-label"> contenenti solo testo seguite subito da un input con @FieldInput
	re := regexp.MustCompile(textLabelRegex + `\s*(<input.*?>)`)
	text = re.ReplaceAllStringFunc(text, func(m string) string {
		parts := re.FindStringSubmatch(m)
		doc, err := html.Parse(strings.NewReader(parts[3]))
		if err == nil {
			var model string
			input := doc.FirstChild.LastChild.FirstChild
			for i := len(input.Attr) - 1; i >= 0; i-- {
				switch input.Attr[i].Key {
				case "class":
					input.Attr[i].Val = strings.ReplaceAll(strings.ReplaceAll(input.Attr[i].Val, "form-check-input", ""), "form-control", "")
					if strings.TrimSpace(input.Attr[i].Val) == "" {
						input.Attr = slices.Delete(input.Attr, i, i+1)
					}
				case "ng-model":
					model = input.Attr[i].Val
					input.Attr = slices.Delete(input.Attr, i, i+1)
				case "ng-maxlength", "type":
					input.Attr = slices.Delete(input.Attr, i, i+1)
				}
			}

			label := parts[2]
			if !strings.Contains(label, "Lang") {
				label = "Lang(c,`" + label + "`)"
			}
			if strings.HasPrefix(model, "$ctrl.model.") {
				field := strings.TrimPrefix(model, "$ctrl.model.")
				result := parts[1] + "@FieldInput(" + label + ", `" + field + "`"
				if len(input.Attr) > 0 {
					result += `, ` + templAttrs(input.Attr)
				}
				result += `)`
				return result
			}
		} else {
			fmt.Println(parts[0], err)
		}
		return parts[0]
	})

	// Rimpiazza le <label class="form-control-label"> contenenti solo testo con @Label
	re = regexp.MustCompile(textLabelRegex)
	text = re.ReplaceAllStringFunc(text, func(m string) string {
		parts := re.FindStringSubmatch(m)
		content := strings.TrimSpace(parts[2])
		if !strings.Contains(content, "Lang") {
			content = "Lang(c,`" + content + "`)"
		}
		if !strings.HasPrefix(content, "{") && !strings.HasSuffix(content, "}") {
			content = "{" + content + "}"
		}
		return parts[1] + `@Label(){ ` + content + ` }`
	})
	return text
}

func (t *Transformer) replaceNext(tag string, at int, with string) {
	key := fmt.Sprintf("%s:%v", tag, at)
	if replacements, ok := t.replace[key]; ok {
		t.replace[key] = append([]string{with}, replacements...)
	} else {
		t.replace[key] = []string{with}
	}
}
