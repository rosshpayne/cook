package main

import (
	_ "encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/cook/global"

	_ "github.com/aws/aws-sdk-go/aws"
	_ "github.com/aws/aws-sdk-go/aws/credentials"

	_ "github.com/aws/aws-lambda-go/lambdacontext"
)

var errExceedEOL error = errors.New("Exceed EOL")
var errFailValidation error = errors.New("Failed Session Validation")

//TODO
// change float32 to float64 as this is what dynamoAttribute.Unmarshal uses
type PrepTask int

const (
	prep PrepTask = iota
	task
)
const (
	EOT int = iota
	NOTEOT
)
const jsonKey string = "task"

func (p PrepTask) string() string {
	switch p {
	case prep:
		return "Prep"
	case task:
		return "Task"
	}
	return "Error"
}

type respT struct {
	Error error
	msg   string
}

//TODO: activate Recipe type here. Also incorporate AltIngrd into output.
// Recipe Record
//
// type PartT struct {
// 	Index string `json:"Idx"`   // short name which is attached to each activity JSON.
// 	Title string `json:"Title"` // long name which is printed out in the ingredients listing
// 	Start int    `json:"Start"` // SortK value in T-?-? that has first instruction for the partition
// }
type RecipeT struct {
	PKey  string `json:"PKey"`
	SortK int    `json:"PKey"`
	RName string `json:"RName"`
	//Title  string   `json:"Title"`
	Index    []string `json:"Index"`
	Serves   string   `json:"Srv"`
	Part     []PartT  `json:"Part"` // order list of recipe parts. Load will prepend "nopart_" if parts are detected in Activities and some are not assigned part.
	Division []PartT  `json:"Div"`  // order list of recipe divisions. Divisions apply at the instruction level rather than ingredient. Example, All instructions that can be done the day-before.
}

// type Part struct {
// 	RName string `json:"RName"`
// 	//Title  string   `json:"Title"`
// 	Index  []string `json:"Index"`
// 	Serves string   `json:"Srv"`
// 	Part   []string `json:"Part"`
// 	IPart  []string `json:"IPart"`
// }

type MeasureCT struct {
	Quantity  string `json:"qty"`
	Size      string `json:"size"`
	Shape     string `json:"shape"` // typically, round, square, rect
	Dimension string `json:"dim"`
	Height    string `json:"height"`
	Unit      string `json:"unit`
}

type taskT struct {
	Type      PrepTask // Prep or Task Activity
	Idx       int      // slice index
	Activityp *Activity
}

// type unit.UnitPI interface {
// 	UPlural() bool
// }

type MeasureT struct {
	Unit     string `json:"unit"` // unit of measure, g, kg, cm, ml, litre, pinch, clove, etc.
	Num      string `json:"num"`  // instance x quantity
	Quantity string `json:"qty"`  // weight, volume, dimension
	Size     string `json:"size"` // large, medium, small
	// normalized quantity
	nzQty float64
}

type PerformT struct {
	//	type      PrepTask q // Prep or Task Activity
	id          int       // order as appears in Activity JSON
	Text        string    `json:"txt"` // original from db - contains {tag}
	text        string    // has {tag} replaced with actual activity attribute
	Verbal      string    `json:"say"` // original from db - contains {tag}
	verbal      string    // has {tag} replaced with actual activity attribute
	Label       string    `json:"label"`
	IngredientS []string  `json:"ingrd"` // case where ingredient prepping produces other ingredients e.g. separating eggs
	Time        float32   `json:"time"`
	Tplus       float32   `json:"tPlus"`
	Unit        string    `json:"unit"`
	UseDevice   *DeviceT  `json:"useD"`
	Measure     *MeasureT `json:"measure"` // used by those tasks that use some portion of the ingredient.
	WaitOn      int       `json:"waitOn"`  // depenency on other activity to complete
	Division    string    `json:"div"`     // inherit from activity if not present. Recipe division based on instructions/tasks not part ingredient e.g. division: day-before, on-day
	Thread      int       `json:"thrd"`    // inherit from activity if not present. No thread means thread 1.
	MergeThrd   int       `json:"mthrd"`   // task where parallel task (thread) will merge
	//DeviceT
	AddToC   []string `json:"addToC"`
	UseC     []string `json:"useC"`
	SourceC  []string `json:"sourceC"`
	Parallel bool     `json:"parallel"`
	Link     bool     `json:"link"`
	//
	AddToCp  []*Container // it is thought that only one addToC will be used per activity - but lets be flexible.
	UseCp    []*Container // ---"---
	SourceCp []*Container // ---"---
	AllCp    []*Container // all containers (addTo, use, source) get saved here.
}
type Activity struct {
	// Pkey          string     `json:"PKey"`
	AId           int    `json:"SortK"`
	Label         string `json:"label"` // used in container listing rather than using ingredient
	Ingredient    string `json:"ingrd"` //
	Alias         string `json:"alias"` // used to index recipe when Ingredient is not suitable.
	IngrdQualifer string `json:"iQual"` // (append) to ingredient
	QualiferIngrd string `json:"quali"` // prepend  to ingredient.
	//	QualMeasure   string      `json:"qualm"`    // qualifer before measure in ingredients output e.g. <qualm> of <measure> a <ingrd>
	AltIngrd    string      `json:"altIngrd"` // key into Ingredient table - used for alternate ingredients only
	QualMeasure string      `json:"qualm"`    // qualifer before measure in ingredients output e.g. <qualm> of <measure> a <ingrd>
	Measure     *MeasureT   `json:"measure"`
	AltMeasure  *MeasureT   `json:"altMeasure"`
	Part        string      `json:"part"`      // ingredients are aggregated by part
	Invisible   bool        `json:"invisible"` // do not include in ingredients listing.
	Overview    string      `json:"ovv"`
	Coord       [2]float32  // X,Y
	Task        []*PerformT `json:"task"`
	Prep        []*PerformT `json:"prep"`
	Division    string      `json:"div"`  // see division in PerformT.
	Thread      int         `json:"thrd"` // activity belongs to thread. No thread means thread 1. Overrides thread at activity level.
	//	UnitMap       map[string]*Unit
	next     *Activity
	prev     *Activity
	nextTask *Activity
	nextPrep *Activity
}

var activityStart *Activity

type Activities []Activity

// links all activities with Tasks
type taskCtl struct {
	start *Activity // ptr to first task
	cnt   int       // task count
}

var taskctl taskCtl = taskCtl{}

// links all Prep activities
type prepCtl struct {
	start *Activity // ptr to first task
	cnt   int       // task count
}

var prepctl prepCtl = prepCtl{}

func (m *MeasureT) UPlural() bool {
	if len(m.Quantity) > 0 {
		f, err := strconv.ParseFloat(m.Quantity, 32)
		if err != nil {
			s := strings.Split(m.Quantity, "-")
			if len(s) > 1 {
				f, err := strconv.ParseFloat(s[1], 32)
				if err != nil {
					return false
				}
				if f > 1 {
					return true
				}
			}
			return false
		}
		if f > 1 {
			return true
		}
	}
	return false
}

func (t *PerformT) UPlural() bool {
	if t.Time > 1 {
		return true
	} else {
		return false
	}
}

func (d *DeviceT) String() string {
	var s string
	if len(d.Temp) > 0 {
		if len(d.Unit) == 0 {
			panic(fmt.Errorf("No Unit specified for device [%s] with Temp in Activity", d.Type))
		}
		t := strings.Split(d.Temp, "/")
		if len(t) > 1 {
			// for an oven device, a/b means <a><unit> fan/ <b><unit> nofan / setting
			if global.WriteCtx() == global.USay {
				s = t[0] + UnitMap[d.Unit].String() + " or " + t[1] + UnitMap[d.Unit].String() + " fan forced"
			} else {
				s = t[0] + UnitMap[d.Unit].String() + "/" + t[1] + UnitMap[d.Unit].String() + " Fan"
			}
		} else {
			s = d.Temp + UnitMap[d.Unit].String()
		}
	}
	if len(d.Set) > 0 {
		if len(s) > 0 {
			if global.WriteCtx() == global.USay {
				s += " or " + d.Set
			} else {
				s += "/" + d.Set
			}
		} else {
			s = d.Set
		}
	}
	return s
}

//type Container struct {

func (c *Container) String() string {
	var (
		b strings.Builder
	)
	if c.Measure != nil {
		b.WriteString(c.Measure.String() + " ")
	}
	if len(c.Label) > 0 {
		b.WriteString(strings.ToLower(c.Label))
	}
	if len(c.Requirement) > 0 {
		b.WriteString(" " + strings.ToLower(c.Requirement))
	}
	if c.AltMeasure != nil {
		b.WriteString(" or ")
		if c.AltMeasure != nil {
			b.WriteString(c.AltMeasure.String() + " ")
		}
		if len(c.AltLabel) > 0 {
			b.WriteString(c.AltLabel)
		}
	}
	return b.String()
}

func (m *MeasureCT) String() string {
	var b strings.Builder
	if m == nil {
		panic(fmt.Errorf("%s", "Measure is nil in method String() of Container"))
	}
	if len(m.Shape) > 0 {
		b.WriteString(m.Shape + " ")
	}
	if len(m.Dimension) > 0 {
		b.WriteString(m.Dimension)
	}
	if len(m.Height) > 0 {
		b.WriteString("x" + m.Height)
	}
	if len(m.Quantity) > 0 {
		b.WriteString(m.Quantity)
	}
	if len(m.Unit) > 0 {
		b.WriteString(m.Unit)
	}
	if len(m.Size) > 0 {
		b.WriteString(m.Size)
	}
	//fmt.Println(s)
	return b.String()
}

var pIngrdScale float64 = 1.00

func (m *MeasureT) String() string {

	if pIngrdScale > 0.85 {
		return m.FormatString()
	}
	//
	//************ ingredient scaling necessary *************
	//
	const (
		c_pinchof string = "pinch of"
	)
	roundTo5 := func(f float64) float64 {
		if f < 10 {
			return f
		}
		i := int(f/10) * 10
		var q int
		switch int(f) - i {
		case 0, 1, 2:
			q = int(f) - (int(f) - i)
		case 3, 4, 5, 6, 7:
			q = int(f) - (int(f) - i) + 5
		case 8, 9:
			q = int(f) - (int(f) - i) + 10
		}
		return float64(q)
	}

	scaleFraction := func(s string) string {
		// supported fractions: 1/8,1/4,1/2,3/4,1,1.25,1.5,2 2.5, 3
		var (
			n1       string
			fraction string
			f        float64
			fstr     string
		)
		switch len(s) {
		case 4:
			// e.g. 31/2 ie. 3 and one half
			n1 = string(s[0])
			fraction = s[1:]
		default:
			fraction = s
		}
		switch fraction {
		case "1/8":
			f = 0.125
		case "1/4":
			f = 0.25
		case "1/2":
			f = 0.667
		case "3/4":
			f = 0.75
		}
		if len(n1) > 0 {
			n, err := strconv.ParseFloat(n1, 64)
			if err != nil {
				panic(err)
			}
			f += n
		}
		f *= pIngrdScale
		fint, frac := math.Modf(f)
		if frac > 0.875 {
			fint += 1
		} else if frac > 0.625 {
			fstr = "3/4"
		} else if frac > 0.375 {
			fstr = "1/2"
		} else if frac > 0.1875 {
			fstr = "1/4"
		} else if frac > 0.075 {
			fstr = "1/8"
		} else {
			return c_pinchof
		}
		ff := strconv.FormatFloat(fint, 'g', -1, 64)
		if fint == 0 {
			fstr = fstr
			return fstr
		} else {
			fstr = ff + " " + fstr
			return fstr
		}
	}

	scaleFloat := func(s string) string {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			panic(fmt.Errorf("Error: cannot covert Quantity [%s] to float64 in *MeasureT.String()", s))
		}
		qty := f * pIngrdScale
		ff, frac := math.Modf(qty)
		if frac > 0.875 {
			ff += 1
			f = 0.0
		} else if frac > 0.625 {
			f = 0.75
		} else if frac > 0.375 {
			f = 0.5
		} else if frac > 0.1875 {
			f = 0.25
		} else if frac > 0.075 {
			f = 0.125
		} else {
			return c_pinchof
		}
		return strconv.FormatFloat(ff+f, 'g', -1, 64)
	}

	scaleInt := func(s string) string {
		i, err := strconv.Atoi(s)
		if err != nil {
			panic(fmt.Errorf("Error: cannot covert Quantity [%s] to int in *MeasureT.String()", s))
		}
		qty := float64(i) * 10 * pIngrdScale
		qty = roundTo5(qty) / 10
		if qty < 1 {
			if qty > 0.875 {
				s = "1.0"
			} else if qty > 0.625 {
				s = "3/4"
			} else if qty > 0.375 {
				s = "1/2"
			} else if qty > 0.1875 {
				s = "1/4"
			} else if qty > 0.075 {
				s = "1/8"
			} else {
				s = c_pinchof
			}
			return s
		}
		return strconv.FormatFloat(qty, 'g', -1, 64)
	}
	// if len(m.Quantity) > 0 && len(m.Size) > 0 {
	// 	return m.Quantity + " " + m.Size
	// }
	//
	//
	if len(m.Num) > 0 && len(m.Quantity) == 0 {
		//
		// case: only qty is defined
		//
		// fraction defined
		if strings.IndexByte(m.Num, '/') > 0 {
			s := scaleFraction(m.Num)
			mn := &MeasureT{Size: m.Size, Num: s, Unit: m.Unit}
			return mn.FormatString()
		}
		if strings.IndexByte(m.Num, '.') > 0 {
			var mn *MeasureT
			s := scaleFloat(m.Num)
			if s != c_pinchof {
				mn = &MeasureT{Size: m.Size, Num: s, Unit: m.Unit}
			} else {
				mn = &MeasureT{Size: m.Size, Num: s, Unit: m.Unit}
			}
			return mn.FormatString()
		}
		s := scaleInt(m.Num)
		mn := &MeasureT{Size: m.Size, Num: s, Unit: m.Unit}
		return mn.FormatString()
	}
	if len(m.Quantity) > 0 && len(m.Unit) == 0 {
		//
		// case: only qty is defined
		//
		// fraction defined
		if strings.IndexByte(m.Quantity, '/') > 0 {
			s := scaleFraction(m.Quantity)
			mn := &MeasureT{Quantity: s, Size: m.Size, Num: m.Num}
			return mn.FormatString()
		}
		if strings.IndexByte(m.Quantity, '.') > 0 {
			var mn *MeasureT
			s := scaleFloat(m.Quantity)
			if s != c_pinchof {
				mn = &MeasureT{Quantity: s, Size: m.Size, Num: m.Num}
			} else {
				mn = &MeasureT{Quantity: s, Size: m.Size, Num: m.Num}
			}
			return mn.FormatString()
		}
		s := scaleInt(m.Quantity)
		mn := &MeasureT{Quantity: s, Size: m.Size, Num: m.Num}
		return mn.FormatString()
	}
	var (
		f      float64
		qty    float64
		qtyStr string
		part   string
	)

	if len(m.Quantity) > 0 && len(m.Unit) > 0 {
		//
		if strings.IndexByte(m.Quantity, '/') > 0 {
			s := scaleFraction(m.Quantity)
			if s == c_pinchof {
				mn := &MeasureT{Quantity: s, Size: m.Size, Num: m.Num}
				return mn.FormatString()
			} else {
				mn := &MeasureT{Quantity: s, Unit: m.Unit, Size: m.Size, Num: m.Num}
				return mn.FormatString()
			}
		}
		if strings.IndexByte(m.Quantity, '.') > 0 {
			var err error
			f, err = strconv.ParseFloat(m.Quantity, 64)
			if err != nil {
				panic(fmt.Errorf("Error: cannot covert Quantity [%s] to float64 in *MeasureT.String()", m.Quantity))
			}
		} else {
			i, err := strconv.Atoi(m.Quantity)
			if err != nil {
				panic(fmt.Errorf("Error: cannot covert Quantity [%s] to int in *MeasureT.String()", m.Quantity))
			}
			f = float64(i)
		}
		// *1000 as we are to change to smaller unit
		qty = f * pIngrdScale
		if m.Unit == "l" || m.Unit == "cm" || m.Unit == "kg" {
			var unit string
			qty *= 1000
			if qty < 1000 {
				switch m.Unit {
				case "l":
					unit = "ml"
				case "cm":
					unit = "mm"
				case "kg":
					unit = "g"
				}
			} else {
				qty /= 1000
				unit = m.Unit
			}
			qty = roundTo5(qty)
			qtyStr = strconv.FormatFloat(float64(qty), 'g', -1, 64)
			mn := &MeasureT{Quantity: qtyStr, Unit: unit, Size: m.Size, Num: m.Num}
			return mn.FormatString()
		}
		qty = roundTo5(qty)
		fint, frac := math.Modf(qty)
		if frac > .825 {
			fint += 1.0
		} else if frac > .625 {
			part = " 3/4"
			//part = ".75"
		} else if frac > .375 {
			part = " 1/2"
			//part = ".5"
		} else if frac > .175 {
			part = " 1/4"
			//part = ".25"
		} else if frac > .75 {
			part = " 1/8"
			//part = ".125"
		} else {
			part = ""
		}
		ff := strconv.FormatFloat(fint, 'g', -1, 64)
		if qty < 5 {
			if fint == 0 {
				mn := &MeasureT{Quantity: part, Unit: m.Unit, Size: m.Size, Num: m.Num}
				return mn.FormatString()
			} else {
				mn := &MeasureT{Quantity: ff + part, Unit: m.Unit, Size: m.Size, Num: m.Num}
				return mn.FormatString()
			}
		} else {
			mn := &MeasureT{Quantity: ff, Unit: m.Unit, Size: m.Size, Num: m.Num}
			return mn.FormatString()
		}

	}
	return ""
}

func (m *MeasureT) FormatString() string {
	// qty_ := m.Quantity
	// measureReset := func() {
	// 	m.Quantity = qty_
	// }
	// is it  short or long units
	var format string
	if len(m.Unit) > 0 {
		u := UnitMap[m.Unit]
		if u == nil {
			panic(fmt.Errorf("Error: Unit [%s] not registered in UnitMap", m.Unit))
		}
		switch global.WriteCtx() {
		case global.UPrint:
			format = u.Print
		case global.USay:
			format = u.Say
		case global.UDisplay:
			format = u.Display
		}
	}
	// **** String() should ignore num attribute as its handle outside of String()
	// if len(m.Num) > 0 {
	// 	m.Quantity = m.Num + " x " + m.Quantity
	// 	defer measureReset()
	// }
	if len(m.Quantity) > 0 && len(m.Size) > 0 && len(m.Unit) == 0 {
		return m.Quantity + " " + m.Size
	}
	if len(m.Quantity) > 0 && len(m.Unit) > 0 {

		if UnitMap[m.Unit].IsNsu() {
			return m.Quantity + UnitMap[m.Unit].String(m)
		}
		if m.Unit == "tsp" || m.Unit == "tbsp" || m.Unit == "g" || m.Unit == "kg" {
			if (strings.IndexByte(m.Quantity, '/') > 0 || strings.IndexByte(m.Quantity, '.') > 0) && format != "l" {
				return m.Quantity + UnitMap[m.Unit].String(m)
			} else {
				return m.Quantity + UnitMap[m.Unit].String(m)
			}
		}
		if strings.IndexByte(m.Quantity, '/') > 0 || strings.IndexByte(m.Quantity, '.') > 0 || m.Quantity == "1" {
			return m.Quantity + UnitMap[m.Unit].String(m)
		} else {
			//if writeCtx == uSay {
			if global.WriteCtx() == global.USay {
				return m.Quantity + UnitMap[m.Unit].String(m)
			} else {
				return m.Quantity + UnitMap[m.Unit].String(m)
			}
		}
	}
	if len(m.Quantity) > 0 {
		if strings.Index(strings.ToLower(m.Quantity), "drizzle") >= 0 {
			return m.Quantity + " of"
		}
		return m.Quantity
	}
	if len(m.Num) > 0 {
		// Num has been written separately earlier
		if len(m.Unit) > 0 {
			return UnitMap[m.Unit].String(m)
		} else {
			return m.Num
		}
	}
	return "No-Measure"
}

func (a Activity) String() string {
	var s string

	addIngrdQual := func() {
		if len(a.IngrdQualifer) > 0 {
			if len(s) > 0 {
				if a.IngrdQualifer[0] == ',' {
					s += a.IngrdQualifer
				} else {
					s += " " + a.IngrdQualifer
				}
			} else {
				s = a.IngrdQualifer
			}
		}
	}

	addNumber := func() {
		if a.Measure != nil && len(a.Measure.Num) > 0 {
			if len(a.Measure.Quantity) > 0 {
				_, err := strconv.Atoi(a.Measure.Quantity[:1])
				if err == nil {
					s = a.Measure.Num + "x"
					return
				}
			}
			s = a.Measure.Num
		}
	}

	addIngrd := func() {
		if len(s) > 0 {
			s += " " + a.Ingredient
		} else {
			s = a.Ingredient
		}
	}

	addAltIngrdMsure := func() {
		if a.AltMeasure != nil {
			m := a.AltMeasure
			am := &MeasureT{Quantity: m.Quantity, Size: m.Size, Unit: m.Unit, Num: m.Num}
			if len(a.AltIngrd) == 0 {
				if UnitMap[m.Unit].IsNsu() {
					s += " (about " + am.String() + ")"
				} else {
					s += " (" + am.String() + ")"
				}
			} else {
				s += " (or " + am.String() + " " + a.AltIngrd + ")"
			}
		} else if len(a.AltIngrd) > 0 {
			s += " (or " + a.AltIngrd + ")"
		}
	}

	addQualIngrd := func() {
		if len(a.QualiferIngrd) > 0 {
			if len(s) > 0 {
				s += " " + a.QualiferIngrd
			} else {
				s = a.QualiferIngrd
			}
		}
	}

	addMeasure := func() {
		if a.Measure != nil {
			if len(s) > 0 {
				s += a.Measure.String()
			} else {
				s = a.Measure.String()
			}
		}
	}
	//sfmt.Println("string() ", a.AId, a.Ingredient)
	// qualm, qty, unit, quali, i , iqual
	//
	if a.Invisible || len(a.Ingredient) == 0 {
		return ""
	}
	if len(a.QualMeasure) > 0 {
		// [qualm] [measure.num size] [ingrd] ([measure.qty+measure.unit])
		s = strings.TrimRight(a.QualMeasure, " ")
		if s[len(s)-3:] != " of" {
			s += " of"
		}
		addMeasure()
		addIngrd()
		addAltIngrdMsure()
		addIngrdQual()
		return expandLiteralTags(s)
	}
	//
	addNumber()
	addMeasure()
	addQualIngrd()
	addIngrd()
	addAltIngrdMsure()
	addIngrdQual()
	return expandLiteralTags(s)
}

func (as Activities) String(r *RecipeT) string {
	// map of parts containing activities
	partM := make(map[string][]*Activity)
	// find if there are any parts to recipe
	for a := &as[0]; a != nil; a = a.next {
		// aggregate activities by their associated Part - if assigned.
		// a.Part is the part short name. It must match the Idx entry in recipe.Part
		if len(a.Part) > 0 {
			partM[a.Part] = append(partM[a.Part], a)
		} else {
			partM["nopart_"] = append(partM["nopart_"], a)
		}
	}
	var b strings.Builder
	if len(r.Part) == 0 {
		//legacy code - Recipe not divided into parts.
		for _, a := range partM["nopart_"] {
			if s := a.String(); len(s) > 0 {
				fmt.Fprintf(&b, "%s\n", strings.TrimLeft(s, " "))
			}
		}
		return b.String()
	}
	// r.Part is an ordered list of recipe parts
	for _, v := range r.Part {
		if len(v.Title) > 0 {
			fmt.Fprintf(&b, "\n%s\n\n", strings.ToUpper(v.Title))
		}
		for _, a := range partM[v.Index] {
			if s := a.String(); len(s) > 0 {
				fmt.Fprintf(&b, "%s\n", strings.TrimLeft(s, " "))
			}
		}
	}
	return b.String()
}
