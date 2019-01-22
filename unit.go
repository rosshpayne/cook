package main

import (
	"fmt"
)

type Unit struct {
	Slabel  string `json:"SortK"`   // short label
	Llabel  string `json:"llabel"`  // long label
	Print   string `json:"print"`   // short or long label when printing unit in ingredients listing.
	Say     string `json:"say"`     // format in verbal communication
	Display string `json:"display"` // format in display of Alexa device
	Plural  string // determines whether unit can be plural i.e have s appended. Applies to Llabel only.
}

var unitMap map[string]*Unit // populated in getActivity()

var unitS []*Unit = []*Unit{
	&Unit{Slabel: "g", Llabel: "gram", Print: "s", Say: "l", Display: "s"},
	&Unit{Slabel: "kg", Llabel: "kilogram", Print: "s", Say: "l", Display: "s"},
	&Unit{Slabel: "tbsp", Llabel: "tablespoon", Print: "l", Say: "l", Display: "s"},
	&Unit{Slabel: "tsp", Llabel: "teespoon", Print: "l", Say: "l", Display: "l"},
	&Unit{Slabel: "l", Llabel: "litre", Print: "l", Say: "l", Display: "s"},
	&Unit{Slabel: "ml", Llabel: "mill", Print: "s", Say: "l", Display: "s"},
	&Unit{Slabel: "mm", Llabel: "millimeter", Print: "s", Say: "l", Display: "s"},
	&Unit{Slabel: "cup", Llabel: "cup", Print: "s", Say: "l", Display: "s", Plural: "s"},
	&Unit{Slabel: "cm", Llabel: "centimeter", Print: "s", Say: "l", Display: "s"},
	&Unit{Slabel: "m", Llabel: "meter", Print: "l", Say: "l", Display: "s"},
	&Unit{Slabel: "C", Llabel: "Celsius", Print: "s", Say: "l", Display: "s"},
	&Unit{Slabel: "F", Llabel: "Fehrenhite", Print: "l", Say: "l", Display: "s"},
	&Unit{Slabel: "min", Llabel: "minute", Print: "l", Say: "l", Display: "s", Plural: "s"},
	&Unit{Slabel: "sec", Llabel: "second", Print: "l", Say: "l", Display: "s", Plural: "s"},
	&Unit{Slabel: "hr", Llabel: "hour", Print: "l", Say: "l", Display: "l", Plural: "s"},
	&Unit{Slabel: "clove", Llabel: "clove", Print: "l", Say: "l", Display: "l", Plural: "s"},
	&Unit{Slabel: "pinch", Llabel: "pinch", Print: "l", Say: "l", Display: "l", Plural: "es"},
	&Unit{Slabel: "sachet", Llabel: "sachet", Print: "l", Say: "l", Display: "s", Plural: "s"},
}

// String output unit text based on mode represented by package variable writeCtx [package_variable-Unit-mode]
func (u *Unit) String(i ...unitPI) string {
	// mode: Print ingredients
	var format string
	if u == nil {
		panic(fmt.Errorf("%s", "Unit is nil in method (*Unit).String()"))
	}
	switch writeCtx {
	case uPrint, uSay, uDisplay:
	default:
		panic(fmt.Errorf("%s", "write context not set"))
	}
	switch writeCtx {
	case uPrint:
		format = u.Print
	case uSay:
		format = u.Say
	case uDisplay:
		format = u.Display
	}
	switch format {
	case "s":
		switch u.Slabel {
		case "C", "F":
			return "\u00B0" + u.Slabel
		default:
			return u.Slabel
		}
	case "l":
		switch u.Slabel {
		case "C", "F":
			return "\u00B0" + u.Llabel
		default:
			if len(i) > 0 && len(u.Plural) > 0 && i[0].uPlural() {
				return " " + u.Llabel + u.Plural
			} else {
				return " " + u.Llabel
			}
		}
	default:
		return u.Slabel
	}

}

func init() {
	unitMap = make(map[string]*Unit, len(unitS))
	for _, v := range unitS {
		unitMap[v.Slabel] = v
	}
}
