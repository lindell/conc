package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/lindell/conc/conc"
)

func main() {
	before := time.Now()

	ret, err := conc.Map([]string{"10", "30", "40", "80"}, func(val string) (int, error) {
		time.Sleep(time.Second)
		return strconv.Atoi(val)
	}, conc.WithMaxConcurrency(2))

	fmt.Println(ret, err, time.Since(before)) // [10 30 40 80] <nil> 2.006875646s
}
