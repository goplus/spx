# XGo syntax for SPX scripts

Write calls such as `say score, 1` in command style when their result is
unused. Use parentheses for a call with arguments inside an expression,
as in `if touching(Edge) { hide }`. Zero-argument getters can read like
fields, as in `xpos` or `Dragon.ypos`.

Use the script spelling of a Go method, for example `setXYpos` for
`SetXYpos`. Omit suffixes such as `__0` and `__1` on overloaded methods.
XGo selects `Step__0` for `step 32` and `Step__1` for `step 32, 2`.

Declare a helper with `func` and typed parameters. XGo infers the types of
local slice and map literals from their elements. This sprite script shows
the total value of three pickups when clicked.

```spx
func lootScore(items []string) int {
    points := {"coin": 1, "gem": 5}
    total := 0
    for item in items {
        total += points[item]
    }
    return total
}

onClick => {
    items := ["coin", "gem", "coin"]
    say lootScore(items), 1
}
```

`for item in items` yields each value. Use `for i, item in items` when
you also need the zero-based index. A numeric range such as `for i in 1:5`
visits 1 through 4.

Choose the handler's lambda to match the event arguments. For example,
`onKey KeyLeft, => { changeXpos -10 }` handles one key. The list overload
can pass the pressed key to the callback.

```spx
onKey [KeyLeft, KeyRight], key => {
    if key == KeyLeft {
        changeXpos -10
    } else {
        changeXpos 10
    }
}
```
