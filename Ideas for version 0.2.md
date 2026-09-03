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

## MATCH
We must simplify match statements:

The following statements A and B must be equal and valid
```sec
enum SameSite {
    Strict,
    Lax,
    None,
    Unspecified,
}
```
A)
```sec
        match self.SameSite {
            Option.Some(site) where site == SameSite.Strict      => { out = out + "; SameSite=Strict" }
            Option.Some(site) where site == SameSite.Lax         => { out = out + "; SameSite=Lax" }
            Option.Some(site) where site == SameSite.None        => { out = out + "; SameSite=None" }
            Option.Some(site) where site == SameSite.Unspecified => { }
            Option.None => { }
        }
```
B)
```sec
        match self.same_site {
            Option.Some(SameSite.Strict) => { out = out + "; SameSite=Strict" }
            Option.Some(SameSite.Lax)    => { out = out + "; SameSite=Lax" }
            Option.Some(SameSite.None)   => { out = out + "; SameSite=None" }
            Option.Some(SameSite.Unspecified) => {}
            Option.None => {}
        }
```
the where should be used to further qualify a match - not being the primary branch-selector.

## Better practice analysis (pitfall analysis)

We should inform that 
```sec
v = v + 1
``` 
is not the best way - this should be 
```sec
v += 1
```

## Copy move
While it is very important to be stringent when it comes to moving and copying we should NOT become square minded and demand semantic byrocracy 
```sec
return Ok(<- result)
```
would be EXAKTLY one of those places where we need to refrain from demanding <- since the result ANYWAY leaves with a move in return.
