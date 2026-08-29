# Ideas for Sec version 0.2 #



## Declarations
Semantic byrocracy in let col := getColor()
should be changed to col := getColor()

## Type inference

Let us update to support type inference.

```sec
switch m {
    case http.Method.GET:  // 1. Complete byrocracy (current)
    case Method.GET:       // 2. Module inference
    case .GET:             // 3. Type inferens (Odin/Zig/Swift) <-- We'll go with this
    case GET:              // 4. Implicit scope (C#)
}
```

