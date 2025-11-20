Use this tool to request decisions or clarifications from the user mid-execution without breaking conversation flow. This allows you to pause work, get user input on specific choices, then continue with their selection.

## When to Use This Tool

Use this tool when you:
1. **Hit a decision point** - Multiple valid approaches and need user preference
2. **Need clarification** - Ambiguous requirements that could go multiple ways
3. **Require approval** - Want explicit user choice before proceeding
4. **Offer options** - Present trade-offs for user to evaluate

## When NOT to Use This Tool

Skip this tool when:
1. You can infer the right choice from context
2. The decision is purely technical with one clear best practice
3. You're asking open-ended questions (use regular conversation instead)
4. You need to show information rather than get a choice

## Tool Behavior

**Execution Flow:**
1. You call this tool with 1-4 questions
2. **Your execution pauses** - You stop and wait for user input
3. User sees a UI dialog/form and selects option(s)
4. Tool returns with `answers` populated
5. You continue work using the user's choices

**Important:** This tool blocks your execution. Do not call it unless you genuinely need user input to proceed.

## Question Format

Each question must have:
- **question**: Clear question text ending with '?' (e.g., "Which library should we use?")
- **header**: Very short label (max 12 chars) for display (e.g., "Library", "Approach")
- **multiSelect**: Boolean - true for checkboxes, false for radio buttons
- **options**: 2-4 choices, each with:
  - **label**: Short text (1-5 words) the user selects
  - **description**: Explanation of what this option means or its trade-offs

The system automatically adds an "Other" option where users can type a custom response.

## Answer Format

You receive a dictionary mapping question index (as string) to selected value(s):

**Single-select answer:**
```json
{ "0": "tokio" }
```

**Multi-select answer (comma-separated):**
```json
{ "0": "tokio,async-std" }
```

**Multiple questions:**
```json
{ "0": "tokio", "1": "sqlx", "2": "proptest,benchmarks" }
```

**Custom "Other" response:**
```json
{ "0": "__other__:User typed this custom value" }
```

## Examples

### Example 1: Single Choice Question

```json
{
  "questions": [
    {
      "question": "Which async runtime should we use?",
      "header": "Runtime",
      "multiSelect": false,
      "options": [
        {
          "label": "Tokio",
          "description": "Most popular, mature ecosystem, lots of libraries"
        },
        {
          "label": "async-std",
          "description": "Standard library API feel, simpler learning curve"
        },
        {
          "label": "Smol",
          "description": "Lightweight, minimal dependencies, fast compilation"
        }
      ]
    }
  ]
}
```

### Example 2: Multiple Questions

```json
{
  "questions": [
    {
      "question": "Which database driver?",
      "header": "Database",
      "multiSelect": false,
      "options": [
        { "label": "SQLx", "description": "Compile-time checked queries" },
        { "label": "Diesel", "description": "Type-safe ORM with migrations" }
      ]
    },
    {
      "question": "Which test features do you want?",
      "header": "Testing",
      "multiSelect": true,
      "options": [
        { "label": "Property tests", "description": "Using proptest for generative testing" },
        { "label": "Benchmarks", "description": "Criterion.rs performance benchmarks" },
        { "label": "Mutation tests", "description": "Test quality verification with mutants" }
      ]
    }
  ]
}
```

### Example 3: Architecture Decision

```json
{
  "questions": [
    {
      "question": "How should we structure the API migration?",
      "header": "Approach",
      "multiSelect": false,
      "options": [
        {
          "label": "Big bang",
          "description": "Migrate all endpoints at once in one PR"
        },
        {
          "label": "Incremental",
          "description": "Migrate endpoint by endpoint, both systems run in parallel"
        },
        {
          "label": "Strangler",
          "description": "New API alongside old, gradually redirect traffic"
        }
      ]
    }
  ]
}
```

## Best Practices

1. **Be specific**: Ask about concrete choices, not open-ended opinions
2. **Provide context**: Option descriptions should explain trade-offs
3. **Keep headers short**: Max 12 characters for clean UI display
4. **Limit questions**: 1-2 questions is ideal, 4 is maximum
5. **Group related choices**: If asking multiple questions, they should be about the same task
6. **Don't abuse**: Only use when you genuinely need input to proceed

## Edge Cases

- **User cancels**: Tool returns error, you should handle gracefully
- **No selection** (multi-select with 0 choices): Possible if user deselects all
- **Timeout**: Long-idle questions may auto-cancel (implementation-dependent)
- **Custom "Other" response**: User can type anything, validate before using

## Integration with TodoWrite

You can use both tools together:

```
1. Create TodoWrite with your task plan
2. Start working on task 1
3. Hit decision point → Call AskUserQuestion (execution pauses)
4. User responds
5. Update TodoWrite to reflect the choice made
6. Continue with remaining tasks
```

Example:
```
Tasks:
  ✓ Analyze REST endpoints
  ⟳ Design GraphQL schema (waiting for user input)
  ○ Implement resolvers

[AskUserQuestion: "Which GraphQL server?"]
[User selects "async-graphql"]

Tasks:
  ✓ Analyze REST endpoints
  ⟳ Design GraphQL schema (using async-graphql)
  ○ Implement resolvers
```

## Important Notes

- This tool **blocks your execution** until the user responds
- Users see a modal/dialog that requires interaction
- If context is canceled (e.g., user stops you), the tool returns error
- The "Other" option is always available - users can provide custom input
- Multi-select results are comma-separated in a single string
