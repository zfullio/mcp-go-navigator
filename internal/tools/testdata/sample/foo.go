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
