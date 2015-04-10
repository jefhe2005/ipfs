# Hi

these are development templates for the [eg]() tool. tl;dr typesafe, template based refactoring.


```go
// +build ignore

package P

import (
	"fmt"
	"gopkg.in/errgo.v1"
)

func before(msg string, err error) error { return fmt.Errorf(msg, err) }
func after(msg string, err error) error  { return errgo.Notef(err, msg) }
````

```sh
#!/bin/bash

go get -u golang.org/x/tools/cmd/eg

cd $GOPATH/src/github.com/ipfs/go-ipfs

cat errgo2.tpl.go

# find all the files that use fmt.Errorf with ', err)'
grep -R fmt.Errorf . | grep -v Godeps | grep '", err)' | cut -d':' -f1 | while read file
do dirname $file | tr -d .                                       # we just want the dirname
done | sort -u | while read pkg                                  # sort it to get a unique list of those
do eg -t egTpls/errgo2.tpl.go -w github.com/ipfs/go-ipfs/$pkg    # run eg on each of these pkgs
done
```