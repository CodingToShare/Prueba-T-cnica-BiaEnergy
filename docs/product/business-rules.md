# Business Rules

A `BUSINESS RULE` constrains product behavior and is derived from the challenge. Rules state *what* must hold; the analytics that satisfy them are designed in `docs/ai/anomaly-analysis.md`, and numeric thresholds remain open decisions until calibrated.

| ID | Rule | Source |
| --- | --- | --- |
| BR-01 | Every analyzed finding is classified into exactly one of: `REAL_ANOMALY`, `EXPLAINABLE_ANOMALY`, `FALSE_POSITIVE`, `DATA_QUALITY`. | §9, §11 |
| BR-02 | Every finding carries a severity (`HIGH`, `MEDIUM`, `LOW`), a confidence, a reason, and a recommended action. A boolean result alone is insufficient. | §10, §11 |
| BR-03 | A change that a known operational event accounts for must not be escalated as a real anomaly. | §8, §18 |
| BR-04 | A significant deviation with no known explanatory event, corroborated by changes in electrical variables, is a real anomaly and must be escalated. | §9, §18 |
| BR-05 | Data-quality problems (inconsistent or implausible electrical readings) are distinguished from consumption anomalies and reported as `DATA_QUALITY`. | §8, §9, §18 |
| BR-06 | Findings are prioritized so the operator knows which to investigate first. A real anomaly with high severity ranks above other findings. | §2, §18 |
| BR-07 | Every explanation and recommendation is supported by evidence computed from the data; conclusions without evidence are not shown as facts. | §10, §12, §18 |
| BR-08 | The recommended action is coherent with the classification (for example: investigate a real anomaly, validate data for a data-quality problem, validate the operation for an explainable anomaly, do not escalate a false positive). | §11, §18 |
| BR-09 | `expected_results.csv` is never available to the analysis or to the end user. | §19 |

## Illustrative Mapping From The Challenge

The challenge illustrates the semantics with these cases. They are acceptance scenarios, not rules keyed by meter:

| Classification | Typical action (§11) | Illustrative severity |
| --- | --- | --- |
| `REAL_ANOMALY` | Investigate meter and installation | High |
| `DATA_QUALITY` | Validate readings / meter | High |
| `EXPLAINABLE_ANOMALY` | Validate operation | Medium |
| `FALSE_POSITIVE` | Do not escalate | Low |

How severity and priority are computed in general (not by classification alone) is OD-04 and OD-05.

## Engineering Rules Protecting Evaluation Integrity

These are project rules, not challenge business rules, recorded here because they constrain every phase:

- ER-01: Analysis logic never branches on specific meter identifiers, timestamps, or other dataset literals.
- ER-02: The four acceptance scenarios are verified by tests; passing them by special-casing is a defect.
