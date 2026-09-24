# Explainability

How the platform explains its conclusions and recommends actions. Governing decision: ADR-006. Nothing here is implemented yet; prompts and model calls are out of scope until Phase 05.

## 1. Structured Evidence Is The Source Of Truth

Every finding carries evidence computed by the deterministic engine. Text is always derived from evidence, never the other way round. Indicative evidence content:

| Element | Example content |
| --- | --- |
| Baseline | Expected value(s) for the affected hours and the window they were computed from |
| Observed | Observed value(s) for the episode |
| Deviation | Percentage deviation and robust Z-score |
| Persistence | Episode start, duration, share of affected hours |
| Changed variables | Which of consumption, voltage, current, power factor changed, direction and magnitude |
| Consistency signals | Physical-consistency and plausibility results |
| Correlated events | Matched events with type, time, and alignment; or the explicit absence of an explanatory event |
| Classification rationale | Which rule path produced the type |
| Confidence components | The signal values that produced the confidence |
| Provenance | Engine configuration version, run id, explanation source |

The UI must be able to answer, for every finding: **what happened, why it matters, what evidence supports it, and what to do next.**

## 2. Response Expectations

The anomaly contract satisfies §10 (`meter_id`, `anomaly`, `type`, `severity`, `confidence`, `reason`, `recommended_action`) and additionally exposes evidence, priority, and `explanation_source`. The `reason` states the key facts in one sentence with real numbers from evidence (e.g., deviation vs baseline and absence or presence of an explanatory event). The `recommended_action` is coherent with the classification (BR-08).

## 3. Explanation Providers

```text
ExplanationProvider
  input:  finding + structured evidence (read-only)
  output: summary, explanation, recommended-action wording, source
```

- **DeterministicExplanationProvider** — templates per classification filled from evidence. Default and fallback; always available; fully testable.
- **OllamaExplanationProvider** — optional; a local open-source model rewrites the same evidence into more natural operator language. Enabled only by configuration.

## 4. Grounding Constraints For Generative Output

- Input to the model is the structured evidence only; no raw database access, no other meters' data, no secrets.
- The model may rephrase and summarize; it may not add numbers, events, causes, or actions absent from the evidence, and may not change type, severity, confidence, or priority.
- Output is validated before use: required sections present, bounded length, and numeric values checked against evidence. Validation failure falls back to deterministic text.
- Tone: factual and operational; no speculation presented as fact.

## 5. Graceful Fallback

If the provider is not configured, unreachable, slow beyond a bounded timeout, erroring, or fails validation, the deterministic text is used. The run still completes, the fallback is logged and counted as a metric, and `explanation_source` shows which provider produced the text. The UI never depends on the generative provider being available.

## 6. What The LLM Never Decides

Anomaly existence, type, severity, numeric confidence, evidence, baselines, priority, and the recommended action code.
