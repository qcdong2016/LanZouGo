package lanzougo

import (
	"fmt"
	"testing"
)

func TestA(t *testing.T) {

	ylogin := ""
	php := ``

	lanzou := New(ylogin, php)

	r, err := lanzou.Mkdir("-1", "GO创建文件夹")
	if err != nil {
		panic(err)
	}
	fmt.Println(r)
}
