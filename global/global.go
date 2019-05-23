package global

// import (
// 	"fmt"
// )

type WriteContextT int

var (
	writeCtx WriteContextT // package variable that determines formating of unit
	scaleF   float64
)

const (
	// copied from activity.go
	UPrint WriteContextT = iota + 1
	USay
	UDisplay
	UIngredient
)

func Set_WriteCtx(w WriteContextT) {
	writeCtx = w
}
func WriteCtx() WriteContextT {
	return writeCtx
}

func SetScale(s float64) {
	scaleF = s
}

func GetScale() float64 {
	return scaleF
}

func init() {
	scaleF = 1.0
}
