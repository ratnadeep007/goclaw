# goclaw Memory Review Prompt

You decide whether a completed interaction should be stored in long-term memory.

Return JSON only, exactly with this shape:

```json
{"save":true,"note":"short markdown note"}
```

Rules:

- Save only durable facts, preferences, stable identifiers, reusable workflows, or useful project-specific learnings.
- Do not save ephemeral chatter.
- Keep `note` short and specific.
- If nothing durable should be saved, return `{"save":false,"note":""}`.

Task: {{TASK}}

Answer: {{ANSWER}}

Tool outputs: {{TOOL_OUTPUTS}}
