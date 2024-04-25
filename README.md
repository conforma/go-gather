# go-gather

WIP, experimental, sharp edges.

## Examples 

### Copy file to file

```
package main

import (
	"context"
	"fmt"

	"github.com/enteprise-contract/go-gather/gather"
)

func main() {
	ctx := context.Background()
	metadata, err := gather.Gather(ctx, "file:///tmp/foo.txt", "file:///tmp/bar.txt")

	if err != nil {
		fmt.Println("error:", err)
	}

	fmt.Println(metadata.Get())
}
```

Executing the above,

``` bash
$ go run main.go
```
produces the following output:

```
map[path:file:///tmp/baz/bar.txt sha:ef4e93945f5b3d481abe655d6ce3870132994c0bd5840e312d7ac97cde021050 size:71680 timestamp:2024-04-25 09:18:20.978669581 -0400 EDT]
```

### Copy directory to directory
```
package main

import (
	"context"
	"fmt"

	"github.com/enteprise-contract/go-gather/gather"
)

func main() {
	ctx := context.Background()
	metadata, err := gather.Gather(ctx, "file:///tmp/foo", "file:///tmp/bar")

	if err != nil {
		fmt.Println("error:", err)
	}

	fmt.Println(metadata.Describe())
}
```

Executing the above,

``` bash
$ go run main.go
```
produces the following output:

```
map[path:/tmp/bar/ size:0 timestamp:2024-04-25 09:57:20.863229843 -0400 EDT]
```