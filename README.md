# Buildkite If Condition Evaluator

```
# individual terms
true
false

# compare values
1 == 1
true != false
"blah" == 'blah'

# compare function calls
env(FOO) == env(BAR)

# compare function calls to attributes
env(FOO) == type

# nested function calls
env(env(FOO))

# regular expression matches
"v1.0.0" =~ ^v

# parenthesis
((env(TAG) =~ ^v) && (env(BRANCH) = master)) || true
```
