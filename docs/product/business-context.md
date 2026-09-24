# Business Context

## Problem

Operators responsible for electrical installations receive large volumes of hourly meter data. Raw readings do not say which deviations matter, whether a change has a known operational cause, whether the data itself can be trusted, or what to do next. Alerts that cannot be explained or prioritized are ignored.

## Product

The AI Energy Management Platform is an MVP that manages electrical meters and uses analytics plus explainable AI to **detect, explain, prioritize, and recommend actions** on anomalies. It is not a meter CRUD: its value is converting data into an operational decision.

## Questions The Product Must Answer

1. What is happening with the meters?
2. Which readings deviate from their expected behavior?
3. Is each anomaly real, operationally explainable, a false positive, or a data-quality problem?
4. Which one should be investigated first?
5. Why did the system reach that conclusion, and what evidence supports it?
6. What action is recommended?

## Users

The challenge names no roles. The working assumption (AS-001) is an energy/operations analyst who monitors a small fleet of meters and decides what to investigate.

## Success Criterion

The complete cycle `DATA → ANALYSIS → ANOMALY → EXPLANATION → PRIORITIZATION → ACTION` is demonstrable, and an evaluator understands within 5–10 minutes what the AI contributes and why the top-priority item requires attention.

## Evaluation Emphasis

| Area | Points |
| --- | --- |
| Frontend / UX | 20 |
| Backend / API | 20 |
| Data / Analytics | 20 |
| Anomaly detection | 15 |
| AI and explainability | 15 |
| Testing / documentation / quality | 10 |

AI-specific evaluation: detects M-109 (30), prioritizes M-109 (25), avoids treating M-106 as a real anomaly (15), detects M-112 as a data-quality problem (10), explains with evidence (10), recommends a coherent action (10).

Implication for engineering: correctness of classification and prioritization, explainability, and a coherent product experience outweigh breadth of features.

## Constraints

- Three-day delivery window.
- Backend language: Go (challenge requirement). Rest of the stack per accepted ADRs.
- Dataset: 12 meters, 14 days, 4,032 hourly readings; variables consumption, voltage, current, power factor.
- `expected_results.csv` is evaluator-only and unavailable to the model and the end user.
- Deliverables: Git repository, working frontend, working backend, 5–10 minute demo.
