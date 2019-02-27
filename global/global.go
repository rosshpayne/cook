package global

type WriteContextT int

var writeCtx WriteContextT // package variable that determines formating of unit

const (
	// copied from activity.go
	UPrint WriteContextT = iota + 1
	USay
	UDisplay
)

func Set_WriteCtx(w WriteContextT) {
	writeCtx = w
}
func WriteCtx() WriteContextT {
	return writeCtx
}
