package sample

import (
	"fmt"
	"strings"
)

type Foo struct {
	ID int
}

func (f *Foo) DoSomething() string {
	return strings.ToUpper(fmt.Sprint(f.ID))
}

// deadHelper — приватный метод, который нигде не вызывается
func (f *Foo) deadHelper() string {
	return fmt.Sprintf("helper: %d", f.ID)
}

var unusedVar = 100

const unusedConst = "ghost"

type unusedType struct {
	Name string
}
