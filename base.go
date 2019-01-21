package main

import (
	_ "encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	_ "github.com/aws/aws-sdk-go/aws"
	_ "github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"

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

type MeasureCT struct {
	Quantity  string `json:"qty"`
	Size      string `json:"size"`
	Shape     string `json:"shape"` // typically, round, square, rect
	Dimension string `json:"dim"`
	//	Diameter string `json:"diameter"`
	//	Square   string `json:"square"`
	//	Rect     string `json:"rect"`
	Height string `json:"height"`
	Unit   string `json:"unit`
}

type taskT struct {
	Type      PrepTask // Prep or Task Activity
	Idx       int      // slice index
	Activityp *Activity
}

type Container struct {
	// Rid      string     `json:"PKey"`
	Cid        string     `json:"SortK"`
	Label      string     `json:"label"`
	Type       string     `json:"type"`
	Purpose    string     `json:"purpose"`
	Coord      [2]float32 `json:"coord"`
	Measure    *MeasureCT `json:"measure"`
	AltMeasure *MeasureCT `json:"altMeasure"`
	Contains   string     `json:"contents"`
	Message    string     `json:"message"`
	start      int        // first id in recipe tasks where container is used
	last       int        // last id in recipe tasks where container is sourced from or recipe is complete.
	reused     int        // links containers that can be the same physical container.
	Activity   []taskT    // slice of tasks (Prep and Task activites) associated with container
}

type DeviceT struct {
	Type      string `json:"type"`
	Set       string `json:"set"`
	Purpose   string `json:"purpose"`
	Alternate string `json:"alternate"`
	Temp      string `json:"temp"`
	Unit      string `json:"unit"`
}

type PerformT struct {
	//	type      PrepTask q // Prep or Task Activity
	id          int       // order as appears in Activity JSON
	Text        string    `json:"txt"` // original from db - contains {tag}
	text        string    // has {tag} replaced
	Verbal      string    `json:"say"` // original from db - contains {tag}
	verbal      string    // has {tag} replaced
	Label       string    `json:"label"`
	IngredientS []string  `json:"ingrd"` // case where ingredient prepping produces other ingredients e.g. separating eggs
	Time        float32   `json:"time"`
	Tplus       float32   `json:"tPlus"`
	Unit        string    `json:"unit"`
	UseDevice   *DeviceT  `json:"useD"`
	Measure     *MeasureT `json:"measure"` // used by those tasks that use some portion of some ingredient.
	WaitOn      int       `json:"waitOn"`  // depenency on other activity to complete
	//DeviceT
	AddToC   []string     `json:"addToC"`
	UseC     []string     `json:"useC"`
	SourceC  []string     `json:"sourceC"`
	Parallel bool         `json:"parallel"`
	Link     bool         `json:"link"`
	AddToCp  []*Container // it is thought that only one addToC will be used per activity - but lets be flexible.
	UseCp    []*Container // ---"---
	SourceCp []*Container // ---"---
	AllCp    []*Container // all containers (addTo, use, source) get saved here.
}

type MeasureT struct {
	Number   string `json:"num"`  // instances of quantity
	Quantity string `json:"qty"`  // weight, volume, dimension
	Size     string `json:"size"` // large, medium, small
	Unit     string `json:"unit"` // unit of measure, g, kg, cm, ml, litre, pinch, clove, etc.
}

// used for alternative ingredients only
// type IngredientT struct {
// 	Name          string
// 	IngrdQualifer string `json:"iQual"` // (append) to ingredient
// 	QualiferIngrd string `json:"quali"` // prepend  to ingredient.
// 	Type          string `json:"iType"`
// 	Measure       *MeasureT
// }

type Activity struct {
	// Pkey          string     `json:"PKey"`
	AId           int         `json:"SortK"`
	Label         string      `json:"label"`    // used in container listing rather than using ingredient
	Ingredient    string      `json:"ingrd"`    //
	IngrdQualifer string      `json:"iQual"`    // (append) to ingredient
	QualiferIngrd string      `json:"quali"`    // prepend  to ingredient.
	QualMeasure   string      `json:"qualm"`    // qualifer before measure in ingredients output e.g. <qualm> of <measure> a <ingrd>
	AltIngrd      []string    `json:"altIngrd"` // key into Ingredient table - used for alternate ingredients only
	Measure       *MeasureT   `json:"measure"`
	AltMeasure    *MeasureT   `json:"altMeasure"`
	Part          string      `json:"part"`      // ingredients are aggregated by part
	Invisible     bool        `json:"invisible"` // do not include in ingredients listing.
	Overview      string      `json:"ovv"`
	Coord         [2]float32  // X,Y
	Task          []*PerformT `json:"task"`
	Prep          []*PerformT `json:"prep"`
	//	unitMap       map[string]*Unit
	next     *Activity
	prev     *Activity
	nextTask *Activity
	nextPrep *Activity
}

type ContainerMap map[string]*Container

var ContainerM ContainerMap

type DevicesMap map[string]string
type DeviceMap map[string]DeviceT

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

type WriteContextT int

var writeCtx WriteContextT // package variable that determines formating of unit

const (
	uPrint WriteContextT = iota + 1
	uSay
	uDisplay
)

func (d *DeviceT) String() string {
	var s string
	if len(d.Temp) > 0 {
		if len(d.Unit) == 0 {
			panic(fmt.Errorf("No Unit specified for device [%s] with Temp in Activity", d.Type))
		}
		t := strings.Split(d.Temp, "/")
		if len(t) > 1 {
			// for an oven device, a/b means <a><unit> fan/ <b><unit> nofan / setting
			if writeCtx == uSay {
				s = t[0] + unitMap[d.Unit].String() + " or " + t[1] + unitMap[d.Unit].String() + " fan forced"
			} else {
				s = t[0] + unitMap[d.Unit].String() + "/" + t[1] + unitMap[d.Unit].String() + " Fan"
			}
		} else {
			s = d.Temp + unitMap[d.Unit].String()
		}
	}
	if len(d.Set) > 0 {
		if len(s) > 0 {
			if writeCtx == uSay {
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

func (m *MeasureCT) String() string {
	var s string
	if m == nil {
		panic(fmt.Errorf("%s", "Measure is nil in method String() of Container"))
	}
	fmt.Printf("MeascureCT == [%#v}\n", m)
	if len(m.Shape) > 0 {
		s = m.Shape + " "
	}
	if len(m.Dimension) > 0 {
		s += m.Dimension
	}
	if len(m.Height) > 0 {
		s += "x" + m.Height
	}
	if len(m.Unit) > 0 {
		s += m.Unit
	}
	if len(m.Size) > 0 {
		s = m.Size
	}
	fmt.Println(s)
	return s
}

var pIngrdScale float64 = 0.75

func (m *MeasureT) String() string {
	if len(m.Quantity) > 0 && len(m.Size) > 0 {
		return m.Quantity + " " + m.Size
	}
	//
	if pIngrdScale > 0.85 {
		mn := &MeasureT{Quantity: m.Quantity, Unit: m.Unit}
		return mn.FormatString()
	}
	//
	// Quantity scaling necessary ********************************************
	//
	const (
		c_pinchof string = "pinch of"
	)
	roundTo5 := func(f float64) float64 {
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
			fstr = c_pinchof
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
		qty := f * 10 * pIngrdScale
		//qty = math.Round(qty) / 10
		qty = roundTo5(qty) / 10
		return strconv.FormatFloat(qty, 'g', -1, 64)
	}
	scaleInt := func(s string) string {
		i, err := strconv.Atoi(s)
		if err != nil {
			panic(fmt.Errorf("Error: cannot covert Quantity [%s] to int in *MeasureT.String()", s))
		}
		qty := float64(i) * 10 * pIngrdScale
		//qty = math.Round(qty) / 10
		qty = roundTo5(qty) / 10
		return strconv.FormatFloat(qty, 'g', -1, 64)
	}

	//
	//
	if len(m.Quantity) > 0 && len(m.Unit) == 0 {
		//
		// case: only qty is defined
		//
		// fraction defined
		if strings.IndexByte(m.Quantity, '/') > 0 {
			s := scaleFraction(m.Quantity)
			mn := &MeasureT{Quantity: s}
			return mn.FormatString()
		}
		if strings.IndexByte(m.Quantity, '.') > 0 {
			s := scaleFloat(m.Quantity)
			mn := &MeasureT{Quantity: s}
			return mn.FormatString()
		}
		s := scaleInt(m.Quantity)
		mn := &MeasureT{Quantity: s}
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
				mn := &MeasureT{Quantity: s}
				return mn.FormatString()
			} else {
				mn := &MeasureT{Quantity: s, Unit: m.Unit}
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
		// *1000 as we are about to change to smaller unit
		qty = f * 1000 * pIngrdScale
		if qty < 1000 && (m.Unit == "l" || m.Unit == "cm" || m.Unit == "kg") {
			var unit string
			switch m.Unit {
			case "l":
				unit = "ml"
			case "cm":
				unit = "mm"
			case "kg":
				unit = "g"
			default:
				unit = m.Unit
			}
			//qty = math.Round(qty*10) / 10
			fmt.Println("roundto5: ", qty)
			qty = roundTo5(qty)
			fmt.Println("roundto5: ", qty)
			qtyStr = strconv.FormatFloat(float64(qty), 'g', -1, 64)
			mn := &MeasureT{Quantity: qtyStr, Unit: unit}
			return mn.FormatString()
		}
		qty /= 1000
		//qty = math.Round(qty*10) / 10
		fmt.Println("roundto5: ", qty)
		qty = roundTo5(qty)
		fmt.Println("roundto5: ", qty)
		fint, frac := math.Modf(qty)
		if frac > .825 {
			fint += 1
			part = ""
		} else if frac > .625 {
			part = "3/4"
			//part = ".75"
		} else if frac > .375 {
			part = "1/2"
			//part = ".5"
		} else if frac > .175 {
			part = "1/4"
			//part = ".25"
		} else if frac > .75 {
			part = "1/8"
			//part = ".125"
		} else {
			part = ""
		}
		ff := strconv.FormatFloat(fint, 'g', -1, 64)
		if qty < 5 {
			if fint == 0 {
				mn := &MeasureT{Quantity: part, Unit: m.Unit}
				return mn.FormatString()
			} else {
				mn := &MeasureT{Quantity: ff + " " + part, Unit: m.Unit}
				return mn.FormatString()
			}
		} else {
			mn := &MeasureT{Quantity: ff, Unit: m.Unit}
			return mn.FormatString()
		}

	}
	return ""
}

func (m *MeasureT) FormatString() string {
	// is it  short or long units
	var format string
	if len(m.Unit) > 0 {
		u := unitMap[m.Unit]
		switch writeCtx {
		case uPrint:
			format = u.Print
		case uSay:
			format = u.Say
		case uDisplay:
			format = u.Display
		}
	}

	if len(m.Quantity) > 0 && len(m.Size) > 0 {
		return m.Quantity + " " + m.Size
	}
	if len(m.Quantity) > 0 && len(m.Unit) > 0 {
		fmt.Printf("FormatString: [%#v]\n", unitMap)
		if m.Unit == "tsp" || m.Unit == "tbsp" || m.Unit == "g" || m.Unit == "kg" {
			if (strings.IndexByte(m.Quantity, '/') > 0 || strings.IndexByte(m.Quantity, '.') > 0) && format != "l" {
				return m.Quantity + " " + unitMap[m.Unit].String()
			} else {
				return m.Quantity + unitMap[m.Unit].String()
			}
		}
		if strings.Index(strings.ToLower(m.Unit), "clove") >= 0 {
			if strings.IndexByte(m.Quantity, '/') > 0 || strings.IndexByte(m.Quantity, '.') > 0 || m.Quantity == "1" {
				return m.Quantity + " " + m.Unit + " of"
			} else {
				return m.Quantity + " " + m.Unit + "s" + " of"
			}
		}
		if strings.Index(strings.ToLower(m.Unit), "bunch") >= 0 {
			if strings.IndexByte(m.Quantity, '/') > 0 || strings.IndexByte(m.Quantity, '.') > 0 || m.Quantity == "1" {
				return m.Quantity + " " + m.Unit + " of"
			} else {
				return m.Quantity + " " + m.Unit + "es" + " of"
			}
		}
		if strings.Index(strings.ToLower(m.Unit), "sachet") >= 0 {
			if strings.IndexByte(m.Quantity, '/') > 0 || strings.IndexByte(m.Quantity, '.') > 0 || m.Quantity == "1" {
				return m.Quantity + " " + m.Unit + " of"
			} else {
				return m.Quantity + " " + m.Unit + "s" + " of"
			}
		}
		if strings.IndexByte(m.Quantity, '/') > 0 || strings.IndexByte(m.Quantity, '.') > 0 || m.Quantity == "1" {
			return m.Quantity + " " + unitMap[m.Unit].String()
		} else {
			if writeCtx == uSay {
				return m.Quantity + " " + unitMap[m.Unit].String() + "s"
			} else {
				return m.Quantity + unitMap[m.Unit].String()
			}
		}
	}
	if len(m.Quantity) > 0 {
		if strings.Index(strings.ToLower(m.Quantity), "drizzle") >= 0 {
			return m.Quantity + " of"
		}
		return m.Quantity
	}
	return "No-Measure"
}

func (a Activity) String() string {
	var s string
	//sfmt.Println("string() ", a.AId, a.Ingredient)
	// qualm, qty, unit, quali, i , iqual
	//
	if a.Invisible || len(a.Ingredient) == 0 {
		return ""
	}
	if len(a.QualMeasure) > 0 {
		s = a.QualMeasure
		if s[len(s)-3:] != " of" {
			s += " of"
		}
	}
	if a.Measure != nil {
		if len(s) > 0 {
			s += " " + a.Measure.String()
		} else {
			s = a.Measure.String()
		}
	}
	if len(a.QualiferIngrd) > 0 {
		if len(s) > 0 {
			s += " " + a.QualiferIngrd
		} else {
			s = a.QualiferIngrd
		}
	}
	if len(s) > 0 {
		s += " " + a.Ingredient
	} else {
		s = a.Ingredient
	}
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
	return s
}

func (as Activities) String() string {
	partM := make(map[string][]*Activity)
	// find if there are any parts to recipe
	for i := 0; i < len(as); i++ {
		a := as[i]
		if len(a.Part) > 0 {
			partM[a.Part] = append(partM[a.Part], &a)
		} else {
			partM["nopart_"] = append(partM["nopart_"], &a)
		}
	}
	var b strings.Builder
	for k, v := range partM {
		if k == "nopart_" {
			continue
		}
		fmt.Fprintf(&b, "\n\n%s\n\n", k)
		for _, a := range v {
			if s := a.String(); len(s) > 0 {
				fmt.Fprintf(&b, "%s\n", strings.TrimLeft(s, " "))
			}
		}
	}
	for _, a := range partM["nopart_"] {
		if s := a.String(); len(s) > 0 {
			fmt.Fprintf(&b, "%s\n", strings.TrimLeft(s, " "))
		}
	}
	return b.String()
}

func (s *sessCtx) loadIngredients() (Activities, error) {
	//
	// Table:  Activity
	//
	kcond := expression.KeyEqual(expression.Key("PKey"), expression.Value("A-"+s.pkey))
	expr, err := expression.NewBuilder().WithKeyCondition(kcond).Build()
	if err != nil {
		return nil, fmt.Errorf("Error: in getIngredientData Query - %s", err.Error())
	}
	input := &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}
	input = input.SetTableName("Recipe").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
	//*dynamodb.DynamoDB,
	result, err := s.dynamodbSvc.Query(input)
	if err != nil {
		return nil, fmt.Errorf("Error: in getIngredientData Query - %s", err.Error())
	}
	if int(*result.Count) == 0 {
		return nil, fmt.Errorf("No data found for reqRId %s in getIngredientData for Activity - ", s.pkey)
	}
	//ActivityS := make([]Activity, int(*result.Count))
	ActivityS := make(Activities, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &ActivityS)
	if err != nil {
		return nil, fmt.Errorf("** Error during UnmarshalListOfMaps in getIngredientData - %s", err.Error())
	}
	//
	// Table:  Unit
	//
	//proj := expression.NamesList(expression.Name("slabel"), expression.Name("llabel"), expression.Name("print"), expression.Name("desc"), expression.Name("say"), expression.Name("display"))
	// kcond = expression.KeyEqual(expression.Key("PKey"), expression.Value("U"))
	// expr, err = expression.NewBuilder().WithKeyCondition(kcond).Build()
	// if err != nil {
	// 	return nil, fmt.Errorf("%s", "Error in expression build of unit table: "+err.Error())
	// }
	// // Build the query input parameters
	// input = &dynamodb.QueryInput{
	// 	KeyConditionExpression:    expr.KeyCondition(),
	// 	FilterExpression:          expr.Filter(),
	// 	ExpressionAttributeNames:  expr.Names(),
	// 	ExpressionAttributeValues: expr.Values(),
	// }
	// input = input.SetTableName("Ingredient").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
	// //*dynamodb.DynamoDB,
	// resultS, err := s.dynamodbSvc.Query(input)
	// if err != nil {
	// 	return nil, fmt.Errorf("Error: in getIngredientData Query - %s", err.Error())
	// }
	// if int(*result.Count) == 0 {
	// 	return nil, fmt.Errorf("No Unit data found ")
	// }
	// //
	// // Note: unitMap is a package variable
	// //
	// unitMap = make(map[string]*Unit, int(*result.Count))
	// unit := make([]*Unit, int(*result.Count))
	// err = dynamodbattribute.UnmarshalListOfMaps(resultS.Items, &unit)
	// if err != nil {
	// 	return nil, fmt.Errorf("%s", "Error in UnmarshalMap of container table: "+err.Error())
	// }
	// for _, v := range unit {
	// 	unitMap[v.Slabel] = v
	// }
	// for k, v := range unitMap {
	// 	fmt.Printf("%s - %#v\n", k, v)
	// }
	//unit = nil

	return ActivityS, nil
}

func (s *sessCtx) loadBaseRecipe() error {
	//
	// Table:  Activity
	//
	kcond := expression.KeyEqual(expression.Key("PKey"), expression.Value("A-"+s.pkey))
	expr, err := expression.NewBuilder().WithKeyCondition(kcond).Build()
	if err != nil {
		panic(err)
	}
	input := &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}
	input = input.SetTableName("Recipe").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
	//*dynamodb.DynamoDB,
	result, err := s.dynamodbSvc.Query(input)
	if err != nil {
		return fmt.Errorf("Error: in readBaseRecipeForContainers Query - %s", err.Error())
	}
	if int(*result.Count) == 0 {
		return fmt.Errorf("No data found for reqRId %s in processBaseRecipe for Activity - ", s.pkey)
	}
	//ActivityS := make([]Activity, int(*result.Count))
	ActivityS := make(Activities, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &ActivityS)
	if err != nil {
		return fmt.Errorf("** Error during UnmarshalListOfMaps in processBaseRecipe - %s", err.Error())
	}
	//
	// Create maps based on AId, Ingredient (plural and singular) and Label (plural and singular)
	//
	idx := 0
	//TODO does id get used??
	s.activityS = ActivityS
	for _, p := range ActivityS {
		for _, p := range p.Prep {
			idx++
			p.id = idx
		}
		for _, p := range p.Task {
			idx++
			p.id = idx
		}
	}
	//
	activityStart = &ActivityS[0]
	ActivityM := make(map[string]*Activity)
	IngredientM := make(map[string]*Activity)
	LabelM := make(map[string]*Activity)
	for i, v := range ActivityS {
		aid := strconv.Itoa(v.AId)
		ActivityM[aid] = &ActivityS[i]
		if len(v.Ingredient) > 0 {
			ingrd := strings.ToLower(v.Ingredient)
			IngredientM[ingrd] = &ActivityS[i]
			if ingrd[len(ingrd)-1] == 's' {
				// make singular entry as well
				IngredientM[ingrd[:len(ingrd)-1]] = &ActivityS[i]
			}
		}
		if len(v.Label) > 0 {
			label := strings.ToLower(v.Label)
			LabelM[label] = &ActivityS[i]
			if label[len(label)-1] == 's' {
				// make singular entry as well
				LabelM[label[:len(label)-1]] = &ActivityS[i]
			}
		}
	}
	// link activities together via next, prev, nextTask, nextPrep pointers. Order in ActivityS is sorted from dynamodb sort key.
	// not sure how useful have next, prev pointers will be but its easy to setup so keep for time being. Do use prev in other part of code.
	for i := 0; i < len(ActivityS)-1; i++ {
		ActivityS[i].next = &ActivityS[i+1]
		if i > 0 {
			ActivityS[i].prev = &ActivityS[i-1]
		}
	}
	//
	// link Task Activities - taskctl is a package variable.
	//
	var j int
	for i, v := range ActivityS {
		if v.Task != nil {
			taskctl.start = &ActivityS[i]
			j = i
			taskctl.cnt++
			for i := j + 1; i < len(ActivityS); i++ {
				if len(ActivityS[i].Task) > 0 {
					ActivityS[j].nextTask = &ActivityS[i]
					j = i
					taskctl.cnt++
				}
			}
			break
		}
	}
	//
	// link Prep Activities - prepctl is a package variable.
	//
	for i, v := range ActivityS {
		if v.Prep != nil {
			prepctl.start = &ActivityS[i]
			j = i
			prepctl.cnt++
			for i := j + 1; i < len(ActivityS); i++ {
				if len(ActivityS[i].Prep) > 0 {
					ActivityS[j].nextPrep = &ActivityS[i]
					j = i
					prepctl.cnt++
				}
			}
			break
		}
	}
	//
	//
	// Parse Activity and generate Containers
	//  If C-0-0 type container then one its a single-activity-container (SAC) ie. a single-ingredient-container (SIC)
	//  if not a member of C-0-0 then maybe shared amoung activities.
	//
	// Table:  Container
	//
	kcond = expression.KeyEqual(expression.Key("PKey"), expression.Value("C-"+s.pkey))
	expr, err = expression.NewBuilder().WithKeyCondition(kcond).Build()
	if err != nil {
		panic(err)
	}
	input = &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		//		ProjectionExpression:      expr.Projection(),
	}
	input = input.SetTableName("Ingredient").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
	//
	result, err = s.dynamodbSvc.Query(input)
	if err != nil {
		return fmt.Errorf("%s", "Error in Query of container table: "+err.Error())
	}
	if int(*result.Count) == 0 {
		fmt.Println("No container data..")
	}
	// Container lookup - given Cid give me pointer to the continer.
	ContainerM = make(ContainerMap, int(*result.Count))
	itemc := make([]*Container, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &itemc)
	if err != nil {
		return fmt.Errorf("%s", "Error in UnmarshalMap of container table: "+err.Error())
	}
	for _, v := range itemc {
		ContainerM[v.Cid] = v
	}
	// for k, v := range ContainerM {
	// 	fmt.Printf("%s - %#v\n", k, v)
	// }
	// common containers - not recipe specific
	kcond = expression.KeyEqual(expression.Key("PKey"), expression.Value("C-0-0"))
	expr, err = expression.NewBuilder().WithKeyCondition(kcond).Build()
	if err != nil {
		panic(err)
	}
	input = &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		//		ProjectionExpression:      expr.Projection(),
	}
	input = input.SetTableName("Ingredient").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
	//
	result, err = s.dynamodbSvc.Query(input)
	if err != nil {
		return fmt.Errorf("%s", "Error in Query of container table: "+err.Error())
	}
	if int(*result.Count) == 0 {
		fmt.Println("No container data..")
	}
	ContainerSAM := make(ContainerMap, int(*result.Count))
	itemc = nil
	itemc = make([]*Container, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &itemc)
	if err != nil {
		return fmt.Errorf("%s", "Error in UnmarshalMap of container table: "+err.Error())
	}
	for _, v := range itemc {
		ContainerSAM[v.Cid] = v
	}
	// for k, v := range ContainerSAM {
	// 	fmt.Printf("%s - %#v\n", k, v)
	// }
	itemc = nil
	//
	// Table:  Unit
	//
	// kcond = expression.KeyEqual(expression.Key("PKey"), expression.Value("U"))
	// expr, err = expression.NewBuilder().WithKeyCondition(kcond).Build()
	// if err != nil {
	// 	return fmt.Errorf("%s", "Error in expression build of unit table: "+err.Error())
	// }
	// // Build the query input parameters
	// input = &dynamodb.QueryInput{
	// 	KeyConditionExpression:    expr.KeyCondition(),
	// 	FilterExpression:          expr.Filter(),
	// 	ExpressionAttributeNames:  expr.Names(),
	// 	ExpressionAttributeValues: expr.Values(),
	// }
	// input = input.SetTableName("Ingredient").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
	// //*dynamodb.DynamoDB,
	// resultS, err := s.dynamodbSvc.Query(input)
	// if err != nil {
	// 	return fmt.Errorf("Error: in getIngredientData Query - %s", err.Error())
	// }
	// if int(*result.Count) == 0 {
	// 	return fmt.Errorf("No Unit data found ")
	// }
	// //
	// // Note: unitMap is a package variable
	// //
	// unitMap = make(map[string]*Unit, int(*result.Count))
	// unit := make([]*Unit, int(*result.Count))
	// err = dynamodbattribute.UnmarshalListOfMaps(resultS.Items, &unit)
	// if err != nil {
	// 	return fmt.Errorf("%s", "Error in UnmarshalMap of container table: "+err.Error())
	// }
	// for _, v := range unit {
	// 	unitMap[v.Slabel] = v
	//}
	// for k, v := range unitMap {
	// 	fmt.Printf("%s - %#v\n", k, v)
	// }
	//unit = nil
	//
	//  Post fetch processing - assign container pointers in Activity and validate that all containers referenced exist
	//
	// for all prep, tasks
	//
	// parse Activities for containers.  Dynamically create single-activity containers and add to ContainerM as required.
	//
	for _, l := range []PrepTask{prep, task} {
		for ap := activityStart; ap != nil; ap = ap.next {
			var p []*PerformT
			switch l {
			case task:
				p = ap.Task
			case prep:
				p = ap.Prep
			}
			if len(p) == 0 {
				continue
			}
			// now compare contains defined in each activity with those registered for
			// the recipe and those that are single-activity-containers
			for idx, p := range p {
				// a prep or task in order specified in JSON not listed in dynamo - beware
				if len(p.AddToC) > 0 {
					//  containers are held in []string
					// check if container is registered or must be dynamically created
					for i := 0; i < len(p.AddToC); i++ {
						// ContainerM contains registered containers
						cId, ok := ContainerM[strings.TrimSpace(p.AddToC[i])]
						if !ok {
							// ContainerSAM contains single activity containers
							// format: <SA?>.<purpose>
							sac := strings.Split(strings.TrimSpace(p.AddToC[i]), ".")
							p.AddToC[i] = sac[0]
							if cId, ok = ContainerSAM[sac[0]]; !ok {
								// is not a single ingredient container or not a registered container
								fmt.Printf("Error:   Container [%s] not found for %s %d\n", strings.TrimSpace(p.AddToC[i]), ap.Label, ap.AId)
								continue
							}
							// Single-Activity-Containers are not pre-configured by the user into the Container repo - to make life easier.
							// dynamically create a container with a new Cid, and add to ContainerM and update all references to it.
							cs := sac[0] // original Single-activity container name
							c := new(Container)
							c.Cid = p.AddToC[i] + "-" + strconv.Itoa(ap.AId)
							switch len(sac) {
							case 1, 2:
								c.Contains = ap.Ingredient
							default:
								c.Contains = sac[2] // prefer to use label as its bit more informative for container listing.
							}
							c.Measure = cId.Measure
							c.Label = cId.Label
							c.Type = cId.Type
							switch len(sac) {
							case 1:
								c.Purpose = cId.Purpose
							default:
								c.Purpose = sac[1]
							}
							// register container by adding to map
							ContainerM[c.Cid] = c
							// update container id in activity
							p.AddToC[i] = c.Cid
							// search for other references and change its name
							if len(ap.Task) > 0 {
								for _, t := range ap.Task {
									for i := 0; i < len(t.SourceC); i++ {
										if t.SourceC[i] == cs {
											t.SourceC[i] = c.Cid
											break
										}
									}
									for i := 0; i < len(t.UseC); i++ {
										if t.UseC[i] == cs {
											t.UseC[i] = c.Cid
											break
										}
									}
									for i := 0; i < len(t.AddToC); i++ {
										if t.AddToC[i] == cs {
											t.AddToC[i] = c.Cid
											break
										}
									}
								}
							}
							cId = c
						}
						// activity to container edge
						p.AddToCp = append(p.AddToCp, cId)
						p.AllCp = append(p.AllCp, cId)
						// container to activity edge
						associatedTask := taskT{Type: l, Activityp: ap, Idx: idx}
						cId.Activity = append(cId.Activity, associatedTask)
					}
				}

				if len(p.UseC) > 0 {
					for i := 0; i < len(p.UseC); i++ {
						// ContainerM contains registered containers
						cId, ok := ContainerM[strings.TrimSpace(p.UseC[i])]
						if !ok {
							// ContainerSAM contains single activity containers
							sac := strings.Split(strings.TrimSpace(p.UseC[i]), ".")
							p.UseC[i] = sac[0]
							if cId, ok = ContainerSAM[sac[0]]; !ok {
								// is not a single ingredient container or not a registered container
								fmt.Printf("Error:   Container [%s] not found for %s %d\n", strings.TrimSpace(p.UseC[i]), ap.Label, ap.AId)
								continue
							}
							// container referened in activity is a single-activity-container (SAP)
							// manually create container and add to ContainerM and update all references to it.
							cs := sac[0] // original non-activity-specific container name
							c := new(Container)
							c.Cid = p.UseC[i] + "-" + strconv.Itoa(ap.AId)
							switch len(sac) {
							case 0, 1:
								c.Contains = ap.Ingredient
							default:
								c.Contains = sac[2] // prefer to use label as its bit more informative for container listing.
							}
							c.Measure = cId.Measure
							c.Label = cId.Label
							c.Type = cId.Type
							switch len(sac) {
							case 1, 2:
								c.Purpose = cId.Purpose
							default:
								c.Purpose = sac[1]
							}
							// register container by adding to map
							ContainerM[c.Cid] = c
							// update name of container in Activity to <name>-AId
							p.UseC[i] = c.Cid
							// search for other references and change its name
							if len(ap.Task) > 0 {
								for _, t := range ap.Task {
									for i := 0; i < len(t.SourceC); i++ {
										if t.SourceC[i] == cs {
											t.SourceC[i] = c.Cid
											break
										}
									}
									for i := 0; i < len(t.UseC); i++ {
										if t.UseC[i] == cs {
											t.UseC[i] = c.Cid
											break
										}
									}
									for i := 0; i < len(t.AddToC); i++ {
										if t.AddToC[i] == cs {
											t.AddToC[i] = c.Cid
											break
										}
									}
								}
							}
							cId = c
						}
						p.UseCp = append(p.UseCp, cId)
						p.AllCp = append(p.AllCp, cId)
						associatedTask := taskT{Type: l, Activityp: ap, Idx: idx}
						cId.Activity = append(cId.Activity, associatedTask)
					}
				}
				if len(p.SourceC) > 0 {
					// ContainerM contains registered containers
					for i := 0; i < len(p.SourceC); i++ {
						cId, ok := ContainerM[strings.TrimSpace(p.SourceC[i])]
						if !ok {
							// ContainerSAM contains single activity containers
							sac := strings.Split(strings.TrimSpace(p.SourceC[i]), ".")
							p.SourceC[i] = sac[0]
							if cId, ok = ContainerSAM[sac[0]]; !ok {
								// is not a single ingredient container or not a registered container
								fmt.Printf("Error:   Container [%s] not found for %s %d\n", strings.TrimSpace(p.SourceC[i]), ap.Label, ap.AId)
								continue
							}
							// container referened in activity is a single-activity-container (SAP)
							// manually create container and add to ContainerM and update all references to it.
							cs := sac[0] // original non-activity-specific container name
							c := new(Container)
							c.Cid = p.SourceC[i] + "-" + strconv.Itoa(ap.AId)
							switch len(sac) {
							case 1, 2:
								c.Contains = ap.Ingredient
							default:
								c.Contains = sac[2] // prefer to use label as its bit more informative for container listing.
							}
							c.Measure = cId.Measure
							c.Label = cId.Label
							c.Type = cId.Type
							switch len(sac) {
							case 1:
								c.Purpose = cId.Purpose
							default:
								c.Purpose = sac[1]
							}
							// register container by adding to map
							ContainerM[c.Cid] = c
							// update name of container in Activity to <name>-AId
							p.SourceC[i] = c.Cid
							// search for other references and change its name
							if len(ap.Task) > 0 {
								for _, t := range ap.Task {
									for i := 0; i < len(t.SourceC); i++ {
										if t.SourceC[i] == cs {
											t.SourceC[i] = c.Cid
											break
										}
									}
									for i := 0; i < len(t.UseC); i++ {
										if t.UseC[i] == cs {
											t.UseC[i] = c.Cid
											break
										}
									}
									for i := 0; i < len(t.AddToC); i++ {
										if t.AddToC[i] == cs {
											t.AddToC[i] = c.Cid
											break
										}
									}
								}
							}
							cId = c
						}
						p.SourceCp = append(p.SourceCp, cId)
						p.AllCp = append(p.AllCp, cId)
						associatedTask := taskT{Type: l, Activityp: ap, Idx: idx}
						cId.Activity = append(cId.Activity, associatedTask)
					}
				}
			}
		}
	}

	// check container is associated with an activity. if not delete from container map.
	for _, c := range ContainerM {
		fmt.Printf("Container: Id: [%s]  Type: [%s]  Size:[%s]  Label: [%s] \n", c.Cid, c.Type, c.Measure.Size, c.Label)
	}
	for _, c := range ContainerM {
		if len(c.Activity) == 0 {
			delete(ContainerM, c.Cid)
		}
	}

	// populate prep/task id
	for i, p := 0, activityStart; p != nil; p = p.next {
		for _, pp := range p.Prep {
			i++
			pp.id = i
		}
		for _, pp := range p.Task {
			i++
			pp.id = i
		}
	}
	//
	// populate device map using device type as key. Maintains latest attribute values for DeviceT which
	// . can be referenced at any point in txt using {device.<deviceType>.<attribute>}
	//
	var ovenOn bool
	DeviceM := make(DeviceMap)

	for p := activityStart; p != nil; p = p.next {
		for _, pp := range p.Prep {
			if pp.UseDevice != nil {
				dt := *pp.UseDevice
				if dt.Type == "oven" {
					ovenOn = true
				}
				typ := strings.ToLower(dt.Type)
				if dt_, ok := DeviceM[typ]; ok {
					// only preserve attributes that have values
					// NB. DeviceM value is a struct not *struct
					ppU := pp.UseDevice
					if len(ppU.Set) > 0 {
						dt_.Set = ppU.Set
					}
					if len(ppU.Purpose) > 0 {
						dt_.Purpose = ppU.Purpose
					}
					if len(ppU.Alternate) > 0 {
						dt_.Alternate = ppU.Alternate
					}
					if len(ppU.Temp) > 0 {
						dt_.Temp = ppU.Temp
					}
					if len(ppU.Unit) > 0 {
						dt_.Unit = ppU.Unit
					}
					DeviceM[typ] = dt_
					dt = dt_
				} else {
					DeviceM[typ] = dt
				}
				// preserve state of Device for the prep/task id
				key := strconv.Itoa(pp.id) + "-" + dt.Type
				DeviceM[key] = dt
			}
		}
		for _, pp := range p.Task {
			if ovenOn {
				key := strconv.Itoa(pp.id) + "-" + "oven"
				DeviceM[key] = DeviceM["oven"]
			}
			if pp.UseDevice != nil {
				dt := *pp.UseDevice
				if dt.Type == "oven" {
					ovenOn = true
				}
				typ := strings.ToLower(dt.Type)
				if dt_, ok := DeviceM[typ]; ok {
					// only preserve attributes that have values
					// NB. DeviceM value is a struct not *struct
					ppU := pp.UseDevice
					if len(ppU.Set) > 0 {
						dt_.Set = ppU.Set
					}
					if len(ppU.Purpose) > 0 {
						dt_.Purpose = ppU.Purpose
					}
					if len(ppU.Alternate) > 0 {
						dt_.Alternate = ppU.Alternate
					}
					if len(ppU.Temp) > 0 {
						dt_.Temp = ppU.Temp
					}
					if len(ppU.Unit) > 0 {
						dt_.Unit = ppU.Unit
					}
					DeviceM[typ] = dt_
					dt = dt_
				} else {
					DeviceM[typ] = dt
				}
				// preserve state of Device for the Activity
				key := strconv.Itoa(pp.id) + "-" + dt.Type
				DeviceM[key] = dt
			}
		}
	}
	// for k, v := range DeviceM {
	// 	fmt.Printf("DeviceM  %s %v\n", k, v)
	// }
	//
	doubleSpace := strings.NewReplacer("  ", " ")
	//
	const (
		time int = iota
		measure
		device
		text
		voice
	)
	var (
		b       strings.Builder // supports io.Write write expanded text/verbal text to this buffer before saving to Task or Verbal fields
		context int
		str     string
		tclose  int
		topen   int
	)
	//
	//  replace all {tag} in text and verbal for each activity. Ignore Link'd activites - they are only relevant at print time
	//
	var pt []*PerformT
	for _, taskType := range []PrepTask{prep, task} {
		for _, interactionType := range []int{text, voice} {
			for p := activityStart; p != nil; p = p.next {
				switch taskType {
				case prep:
					pt = p.Prep
				case task:
					pt = p.Task
				}
				for _, pt := range pt {
					// perform over slice of preps, tasks
					switch interactionType {
					case text:
						writeCtx = uDisplay // unit formating
						str = strings.TrimLeft(pt.Text, " ")
						s := str[0]
						str = strings.ToUpper(string(s)) + str[1:]
					case voice:
						writeCtx = uSay // unit formating
						str = pt.Verbal
					}
					// if no {} then print and return to top of the loop
					t1 := strings.IndexByte(str, '{')
					if t1 < 0 {
						b.WriteString(str + " ")
						switch interactionType {
						case text:
							pt.text = doubleSpace.Replace(b.String())
						case voice:
							pt.verbal = doubleSpace.Replace(b.String())
						}
						b.Reset()
						continue
					}
					for tclose, topen = 0, strings.IndexByte(str, '{'); topen != -1; {
						var (
							el  string
							el2 string
						)
						p := p
						b.WriteString(str[tclose:topen])
						nextclose := strings.IndexByte(str[topen:], '}')
						if nextclose == -1 {
							return fmt.Errorf("Error: closing } not found in Activity [%d] string [%s] ", p.AId, str)
						}
						nextopen := strings.IndexByte(str[topen+1:], '{')
						if nextopen != -1 {
							if nextclose > nextopen {
								return fmt.Errorf("Error: closing } not found in Activity [%d] string [%s] ", p.AId, str)
							}
						}
						tclose += strings.IndexByte(str[tclose:], '}')
						tclose_ := tclose
						// examine tag to see if it references entities outside of current activity
						//   currenlty only device oven and noncurrent activity is supported
						if tdot := strings.IndexByte(str[topen+1:tclose], '.'); tdot > 0 {
							// dot notation used. Breakdown object being referenced.
							s := strings.SplitN(strings.ToLower(str[topen+1:tclose]), ".", 2)
							el, el2 = s[0], s[1]
							if el == "ingrd" {
								// reference to attribute in noncurrent activity e.g. {ingrd.30}
								p = ActivityM[str[topen+1+tdot+1:tclose]]
								tclose_ -= len(str[topen+1+tdot+1:tclose]) + 1
								//el = str[topen+1 : tclose_]
							}
						} else {
							el, el2 = strings.ToLower(str[topen+1:tclose_]), ""
						}
						switch el {
						case "device":
							s := strings.Split(el2, ".")
							if ov, ok := DeviceM[strconv.Itoa(pt.id)+"-"+s[0]]; ok {
								switch s[1] {
								case "temp":
									if len(ov.Unit) == 0 {
										return fmt.Errorf("in processBaseRecipe. No Unit defined for oven temperature for activity [%d, %d]\n", p.AId, pt.id)
									}
									fmt.Fprintf(&b, "%s", ov.String())
								case "set":
									fmt.Fprintf(&b, "%s", ov.Set)
								case "alternate":
									fmt.Fprintf(&b, "%s", ov.Alternate)
								case "purpose":
									fmt.Fprintf(&b, "%s", ov.Purpose)
								}
							} else {
								return fmt.Errorf("in processBaseRecipe. No device [%s] found for activity [%d, %d]\n", s[0], p.AId, pt.id)
							}
						case "iqual":
							fmt.Fprintf(&b, "%s", p.IngrdQualifer)
						case "quali":
							fmt.Fprintf(&b, "%s", p.QualiferIngrd)
						case "usec", "addtoc":
							var c *Container
							if el == "usec" {
								c = pt.UseCp[0]
							} else {
								c = pt.AddToCp[0]
							}
							m := c.Measure
							// useC.form
							if len(el2) > 0 {
								switch el2 {
								case "form": // depreciated - only alt used now. Still here to support existing
									m = c.Measure
								case "alt":
									m = c.AltMeasure
								default:
									return fmt.Errorf(`Error: useC or addtoC tag not followed by "form" or "alt" type in AId [%d] [%s]`, p.AId, str)
								}
							}
							if m != nil {
								s := m.String()
								fmt.Fprintf(&b, "%s", strings.ToLower(s+" "+c.Label))
							}
						case "measure":
							context = measure
							// is it the task measure
							if pt.Measure != nil {
								fmt.Fprintf(&b, "%s", pt.Measure.String())
								break
							}
							// is it the activity measure
							if p.Measure == nil {
								return fmt.Errorf("in processBaseRecipe. Ingredient measure not defined for Activity [%d]\n", p.AId)
							}
							m := p.Measure
							fmt.Fprintf(&b, "%s", "{"+m.Quantity+"|"+m.Unit+"|"+m.Size+"|"+m.Number+"}")
							//
							//fmt.Fprintf(&b, "%s", p.Measure.String(formatonly))
						case "actmeasure", "ameasure":
							context = measure
							// is it the task measure
							if p.Measure != nil {
								fmt.Fprintf(&b, "%s", p.Measure.String())
							}
						case "qty":
							context = measure
							if p.Measure == nil {
								return fmt.Errorf("in processBaseRecipe. Ingredient measure not defined for Activity [%d]\n", p.AId)
							}
							//fmt.Fprintf(&b, "%s", p.Measure.Quantity)
							fmt.Fprintf(&b, "%s", "{"+p.Measure.Quantity+"}")
							//fmt.Fprintf(&b, "%s", p.Measure.String())
						case "size":
							if p.Measure == nil {
								return fmt.Errorf("in processBaseRecipe. Ingredient measure not defined for Activity [%d]\n", p.AId)
							}
							fmt.Fprintf(&b, "%s", p.Measure.Size)
						case "used":
							if pt.UseDevice == nil {
								return fmt.Errorf("in processBaseRecipe. UseDevice attribute not defined for Activity [%d]\n", p.AId)
							}
							fmt.Fprintf(&b, "%s", strings.ToLower(pt.UseDevice.Type))
							context = device
						case "alternate", "devicealt":
							if pt.UseDevice == nil {
								return fmt.Errorf("in processBaseRecipe. UseDevice attribute not defined for Activity [%d]\n", p.AId)
							}
							fmt.Fprintf(&b, "%s", strings.ToLower(pt.UseDevice.Alternate))
							context = device
						case "qualm":
							fmt.Fprintf(&b, "%s", strings.ToLower(p.QualMeasure))
						case "temp":
							if pt.UseDevice == nil {
								return fmt.Errorf("in processBaseRecipe. UseDevice attribute not defined for Activity [%d]\n", p.AId)
							}
							context = device
							fmt.Fprintf(&b, "%s", pt.UseDevice.String())
						case "label":
							fmt.Fprintf(&b, "%s", pt.Label)
						case "unit":
							var (
								u      *Unit
								ok     bool
								plural bool
							)
							switch context {
							case measure:
								if p.Measure == nil {
									return fmt.Errorf("in processBaseRecipe. Measure not defined for Activity [%d]\n", p.AId)
								}
								m := p.Measure
								if !(strings.IndexByte(m.Quantity, '/') > 0 || strings.IndexByte(m.Quantity, '.') > 0 || m.Quantity == "1") {
									plural = true
								}
								if len(p.Measure.Unit) == 0 {
									return fmt.Errorf("in processBaseRecipe. Unit for time, [%s], not defined for Measure in Activity [%d]\n", pt.Unit, p.AId)
								}
								if u, ok = unitMap[p.Measure.Unit]; !ok {
									return fmt.Errorf("in processBaseRecipe. Unit for measure, [%s], not defined in unitM for Activity [%d]\n", p.Measure.Unit, p.AId)
								}
							case time:
								if pt.Time > 0 {
									plural = true
								}
								if len(pt.Unit) == 1 {
									return fmt.Errorf("in processBaseRecipe. Unit for time, [%s], not defined for Activity [%d]\n", pt.Unit, p.AId)
								}
								if u, ok = unitMap[pt.Unit]; !ok {
									return fmt.Errorf("in processBaseRecipe. Unit for time, [%s], not defined in unitM for Activity [%d]\n", pt.Unit, p.AId)
								}
							}
							if context == device {
								// ignore device as unit now printed with temp tag.
								break
							}
							if plural && interactionType == voice && u.Say == "l" {
								fmt.Fprintf(&b, "%s", u.String()+"s")
							} else {
								fmt.Fprintf(&b, "%s", u.String())
							}
						case "ingrd":
							fmt.Fprintf(&b, "%s", strings.ToLower(p.Ingredient))
						case "time":
							context = time
							fmt.Fprintf(&b, "%2.0f", pt.Time)
						case "tplus":
							{
								context = time
								fmt.Fprintf(&b, "%2.0f", pt.Tplus+pt.Time)
							}
						}
						tclose += 1
						topen = strings.IndexByte(str[tclose:], '{')
						if topen == -1 {
							b.WriteString(str[tclose:])
						} else {
							topen += tclose
						}
					}
					if topen == -1 && strings.IndexByte(str[tclose:], '}') != -1 {
						return fmt.Errorf("Error: closing } found with no open { in Activity [%d] string [%s] ", p.AId, str)
					}
					switch interactionType {
					case text:
						pt.text = doubleSpace.Replace(b.String())
						b.Reset()
					case voice:
						pt.verbal = doubleSpace.Replace(b.String())
						b.Reset()
					}
				}
			}
		}
	}
	//
	//  Generate and save metadata from base Activities to Dyanmodb
	//
	ptS, err := ActivityS.generateAndSaveTasks(s)
	if err != nil {
		return fmt.Errorf("Error in generateAndSaveTasks in processBaseRecipe - %s", err.Error())
	}
	err = s.generateAndSaveIndex(LabelM, IngredientM)
	if err != nil {
		return fmt.Errorf("Error in generateAndSaveIndex in processBaseRecipe  - %s", err.Error())
	}
	//
	// Post processing of Containers
	//
	// find first reference to Container in the ordered instruction (ptS) list
	for _, v := range ContainerM {
		var found bool
		v.start = 99999
		for _, pt := range ptS {
			for _, c := range pt.taskp.AllCp {
				if c == v {
					if c.start > pt.SortK {
						c.start = pt.SortK
						found = true
						break
					}
				}
			}
			if found {
				break
			}
		}
	}
	// find last reference to container in ptS list.
	// in most cases a container is sourced from to represent its last use
	for _, v := range ContainerM {
		var found bool
		for i := len(ptS) - 1; i >= 0; i-- {
			// find last appearance (typically sourceC). Start at last ptS and work backwards
			for _, c := range ptS[i].taskp.AllCp {
				if c == v {
					if ptS[i].SortK > c.last {
						c.last = ptS[i].SortK
						found = true
						break
					}
				}
			}
			if found {
				break
			}
		}
	}
	//
	err = ContainerM.saveContainerUsage(s)
	if err != nil {
		return fmt.Errorf("Error in readBaseRecipe after saveContainerUsage - %s", err.Error())
	}
	DevicesM := make(DevicesMap)
	for p := activityStart; p != nil; p = p.next {
		for _, pp := range p.Prep {
			if pp.UseDevice != nil {
				typ := strings.ToLower(pp.UseDevice.Type)
				if _, ok := DevicesM[typ]; !ok {
					var str string
					pp := pp.UseDevice
					if len(pp.Set) > 0 {
						str = "Set to " + pp.Set + ". "
					}
					if len(pp.Temp) > 0 {
						str = "Set to " + pp.Temp + " " + pp.Unit + ". "
					}
					if len(pp.Purpose) > 0 {
						str += pp.Purpose
					}
					if len(pp.Alternate) > 0 {
						str += " Alternative: " + pp.Alternate
					}
					DevicesM[typ] = str
				}
			}
		}
		for _, pp := range p.Task {
			if pp.UseDevice != nil {
				typ := strings.ToLower(pp.UseDevice.Type)
				if _, ok := DevicesM[typ]; !ok {
					var str string
					pp := pp.UseDevice
					if len(pp.Set) > 0 {
						str = "Set to " + pp.Set + ". "
					}
					if len(pp.Temp) > 0 {
						str = "Set to " + pp.Temp + " " + pp.Unit + ". "
					}
					if len(pp.Purpose) > 0 {
						str += pp.Purpose
					}
					if len(pp.Alternate) > 0 {
						str += "Alternative: " + pp.Alternate
					}
					DevicesM[typ] = str
				}
			}
		}
	}
	err = DevicesM.saveDevices(s)
	if err != nil {
		return fmt.Errorf("Error in readBaseRecipe after saveDevice - %s", err.Error())
	}

	return nil

} //
