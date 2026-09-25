# Explainability

How the platform explains its conclusions and recommends actions. Governing decision: ADR-006. Phase 02 implemented the canonical structured evidence (§1), the deterministic recommended action, and a one-sentence reason built from evidence. Explanation providers, prompts and model calls are Phase 05.

## 1. Structured Evidence Is The Source Of Truth

Every finding carries evidence computed by the deterministic engine (`analysis.Finding`, see `anomaly-analysis.md` §15). Text is always derived from evidence, never the other way round. Explanation providers, deterministic or generative, and any later phase **consume this evidence as-is**. They must not recompute or override the classification, severity, confidence, priority, recommended action, baselines or metrics.

| Element | Engine field (Phase 02) |
| --- | --- |
| Baseline and observed | `Consumption` (baseline and observed energy over the flagged readings, their deviation, median hourly deviation); per-metric median baseline/observed in `Metrics`; per-reading baseline and observed in `Signals` |
| Deviation | Relative deviation and robust Z per signal; median and maximum deviation per metric |
| Persistence | `Persistence`: onset, last flagged reading, duration, flagged/span readings, density, longest run, sustained, recovery (time, readings, post-episode deviation) |
| Changed variables | `Metrics`: for each of consumption, voltage, current, power factor and consumption-to-load ratio — triggered readings, direction, whether it corroborates |
| Consistency signals | The consumption-to-load ratio metric; the `REPEATED_ELECTRICAL_INCONSISTENCY` rule for data-quality findings |
| Correlated events | `RelatedEvents`: type, time, verbatim description, offset from onset, role (`EXPLAINS`, `CORROBORATES`, `CONTEXT`); an empty list is the explicit absence of any nearby event |
| Classification rationale | `Rule`: the decision path that produced the type |
| Confidence components | `ConfidenceDetail`: signal strength, persistence, multivariate support, event context, pattern support |
| Action and reason | `RecommendedAction` code; `Reason`, one deterministic sentence formatted only from the fields above |
| Provenance | Engine configuration version, run id, explanation source: Phase 03 (runs) and Phase 05 (providers) |

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
