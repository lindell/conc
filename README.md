conc
----
Conc is a package to help with concurrent operations in Go. Works with Go1.18 generics.

## Map

Map takes a slice and a function, it then calls the function with each value of the slice
The return of each function will be values in the returned slice

```go
before := time.Now()

ret, err := conc.Map([]string{"10", "30", "40", "80"}, func(val string) (int, error) {
    time.Sleep(time.Second)
    return strconv.Atoi(val)
}, conc.WithMaxConcurrency(2))

fmt.Println(ret, err, time.Since(before)) // [10 30 40 80] <nil> 2.006875646s
```
