package main

import (
	"flag"
	"log"
	"os"
	"text/template"

	"github.com/knakk/rdf"
)

const tmpl = `digraph Ontology {
	node [shape=plaintext];

	{{range .Classes -}}
		"{{.Label}}"[label=<<TABLE BORDER='0' CELLBORDER='1' CELLSPACING='0' CELLPADDING='5'>
			<TR>
				<TD ALIGN='LEFT' BGCOLOR='#e0e0e0'><FONT POINT-SIZE='12' FACE='monospace'>{{.Label}}</FONT><BR ALIGN='LEFT'/></TD>
			</TR>
			{{- range .Literals -}}
			<TR><TD ALIGN='LEFT'><B>{{.}}</B><BR ALIGN='LEFT'/></TD></TR>
			{{end}}
		</TABLE>>];
	{{end}}
	{{range .Links -}}
		"{{.From}}"->"{{.To}}"[label=<<B>{{.Label}}</B>>];
	{{end}}
}
`

func mustURI(s string) rdf.IRI {
	uri, err := rdf.NewIRI(s)
	if err != nil {
		panic(err)
	}
	return uri
}

var (
	rdfsClass    = mustURI("http://www.w3.org/2000/01/rdf-schema#Class")
	rdfsLabel    = mustURI("http://www.w3.org/2000/01/rdf-schema#label")
	rdfsProperty = mustURI("http://www.w3.org/2000/01/rdf-schema#Property")
	rdfsDomain   = mustURI("http://www.w3.org/2000/01/rdf-schema#domain")
	rdfsRange    = mustURI("http://www.w3.org/2000/01/rdf-schema#range")
)

type Class struct {
	Label    string
	Literals []string
}

type Ontology struct {
	Classes []Class
	Links   []Link
}

type Link struct {
	From, To, Label string
}

func main() {
	log.SetPrefix("onto2dot: ")
	log.SetFlags(0)

	dotTmpl := template.Must(template.New("dot").Parse(tmpl))

	inFile := flag.String("in", "", "ontology in RDF turtle format")
	prefLang := flag.String("lang", "no", "prefer labels with this language tag")
	flag.Parse()

	if *inFile == "" {
		flag.Usage()
		os.Exit(2)
	}

	f, err := os.Open(*inFile)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	dec := rdf.NewTripleDecoder(f, rdf.Turtle)
	trs, err := dec.DecodeAll()
	if err != nil {
		log.Fatal(err)
	}

	var (
		classes   = make(map[rdf.IRI]map[rdf.IRI]bool)
		iriLabels = make(map[rdf.IRI]string)
		relations = make(map[rdf.IRI]map[rdf.IRI]bool)
		curSubj   rdf.IRI
		curProp   rdf.IRI
	)

	for _, tr := range trs {
		if rdf.TermsEqual(tr.Obj, rdfsClass) {
			classes[tr.Subj.(rdf.IRI)] = make(map[rdf.IRI]bool)
			curSubj = tr.Subj.(rdf.IRI)
		}
		if rdf.TermsEqual(tr.Subj, curSubj) &&
			rdf.TermsEqual(tr.Pred, rdfsLabel) &&
			tr.Obj.(rdf.Literal).Lang() == *prefLang {
			iriLabels[curSubj] = tr.Obj.(rdf.Literal).String()
		}
		if rdf.TermsEqual(tr.Obj, rdfsProperty) {
			curProp = tr.Subj.(rdf.IRI)
		}
		if rdf.TermsEqual(tr.Subj, curProp) &&
			rdf.TermsEqual(tr.Pred, rdfsLabel) &&
			tr.Obj.(rdf.Literal).Lang() == *prefLang {
			iriLabels[curProp] = tr.Obj.(rdf.Literal).String()
		}
		if rdf.TermsEqual(tr.Pred, rdfsDomain) {
			if _, ok := classes[tr.Obj.(rdf.IRI)]; !ok {
				classes[tr.Obj.(rdf.IRI)] = make(map[rdf.IRI]bool)
			}
			classes[tr.Obj.(rdf.IRI)][tr.Subj.(rdf.IRI)] = true
		}
		if rdf.TermsEqual(tr.Pred, rdfsRange) {
			obj := tr.Obj.(rdf.IRI)
			if _, ok := classes[obj]; ok {
				if _, ok := relations[curProp]; !ok {
					relations[curProp] = make(map[rdf.IRI]bool)
				}
				relations[curProp][obj] = true
			}
		}
	}

	var (
		ont   Ontology
		links []Link
	)

	for class, props := range classes {
		var lits []string
		for lit, _ := range props {
			if rel, ok := relations[lit]; ok {
				for obj, _ := range rel {
					links = append(links, Link{
						From:  iriLabels[class],
						To:    iriLabels[obj],
						Label: iriLabels[lit],
					})
				}
			} else {
				if iriLabels[lit] == "" {
					log.Printf("missing @%s label for %v", *prefLang, lit)
				} else {
					lits = append(lits, iriLabels[lit])
				}
			}
		}
		ont.Classes = append(ont.Classes, Class{
			Label:    iriLabels[class],
			Literals: lits,
		})
		ont.Links = links
	}

	if err := dotTmpl.Execute(os.Stdout, ont); err != nil {
		log.Fatal(err)
	}
}
