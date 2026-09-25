// Fictional API payloads for component tests. Shapes follow the generated
// OpenAPI types; values are invented (meter IDs such as "TST-7" are not from
// the challenge dataset).
import type {
  AnalysisRun,
  AnomalyDetail,
  AnomalyList,
  DashboardSummary,
  Explanation,
  MeterList,
  MeterSummary,
} from "@/lib/api/types";

export const dashboardBefore: DashboardSummary = {
  meters: 3,
  readings: 72,
  total_consumption_kwh: 4321.5,
  period: { first_reading_at: "2030-01-01T00:00:00", last_reading_at: "2030-01-01T23:00:00" },
  anomalies: 0,
  high_priority: 0,
  aggregate_confidence: null,
  latest_analysis: null,
  active_analysis: null,
};

export const dashboardAfter: DashboardSummary = {
  ...dashboardBefore,
  anomalies: 3,
  high_priority: 1,
  aggregate_confidence: 0.71,
  latest_analysis: { id: 7, status: "COMPLETED", completed_at: "2030-02-01T10:20:00Z" },
};

export function run(overrides: Partial<AnalysisRun> = {}): AnalysisRun {
  return {
    analysis_id: 9,
    status: "QUEUED",
    stage: "QUEUED",
    progress: 0,
    engine_version: "1.0.0",
    created_at: "2030-02-01T10:00:00Z",
    started_at: null,
    completed_at: null,
    meters_count: null,
    readings_count: null,
    events_count: null,
    findings_count: null,
    high_priority_count: null,
    aggregate_confidence: null,
    error: null,
    ...overrides,
  };
}

export const completedRun = run({
  status: "COMPLETED",
  stage: "COMPLETED",
  progress: 100,
  started_at: "2030-02-01T10:00:01Z",
  completed_at: "2030-02-01T10:00:02Z",
  meters_count: 3,
  readings_count: 72,
  events_count: 1,
  findings_count: 3,
  high_priority_count: 1,
  aggregate_confidence: 0.71,
});

export function meter(overrides: Partial<MeterSummary> = {}): MeterSummary {
  return {
    meter_id: "TST-1",
    total_consumption_kwh: 1000.25,
    variation_pct: null,
    computed_status: null,
    anomaly_id: null,
    anomaly_type: null,
    severity: null,
    confidence: null,
    priority: null,
    ...overrides,
  };
}

export function meterList(items: MeterSummary[], total = items.length, offset = 0, analyzed = false): MeterList {
  return {
    items,
    pagination: { limit: 10, offset, total },
    analysis: analyzed ? { id: 7, status: "COMPLETED", completed_at: "2030-02-01T10:20:00Z" } : null,
  };
}

export const anomalyList: AnomalyList = {
  items: [
    {
      id: 41,
      analysis_id: 7,
      meter_id: "TST-7",
      priority: 1,
      type: "REAL_ANOMALY",
      severity: "HIGH",
      confidence: 0.884,
      status: "OPEN",
      started_at: "2030-01-12T14:00:00",
      last_observed_at: "2030-01-14T23:00:00",
      duration_hours: 58,
      variation_pct: 95.2,
      reason: "Consumption rose well above its baseline.",
      recommended_action: "INVESTIGATE_METER_AND_INSTALLATION",
    },
  ],
  pagination: { limit: 5, offset: 0, total: 1 },
  analysis: { id: 7, status: "COMPLETED", completed_at: "2030-02-01T10:20:00Z" },
};

export function anomalyDetail(overrides: Partial<AnomalyDetail> = {}): AnomalyDetail {
  return {
    ...anomalyList.items[0],
    rule: "UNEXPLAINED_PERSISTENT_CONSUMPTION_SHIFT",
    created_at: "2030-02-01T10:00:02Z",
    // Findings stored before explanations existed; see explanation() below.
    explanation: null,
    evidence: {
      schema_version: 1,
      rule: "UNEXPLAINED_PERSISTENT_CONSUMPTION_SHIFT",
      evidence_strength: 3.8,
      consumption: { baseline_kwh: 2000, observed_kwh: 3904, deviation_pct: 95.2, median_deviation_pct: 94.1, direction: "UP" },
      persistence: {
        duration_hours: 58,
        flagged_readings: 58,
        span_readings: 58,
        density_fraction: 1,
        longest_run: 58,
        sustained: true,
        recovery: { recovered: false, recovered_at: null, readings_observed: 0, median_consumption_deviation_pct: 0 },
      },
      metrics: [
        {
          metric: "consumption_kwh",
          evaluated_readings: 58,
          triggered_readings: 58,
          median_observed: 90,
          median_baseline: 46,
          median_deviation_pct: 94.1,
          max_abs_deviation_pct: 120,
          direction: "UP",
          corroborates: false,
        },
        {
          metric: "current_a",
          evaluated_readings: 58,
          triggered_readings: 58,
          median_observed: 400,
          median_baseline: 200,
          median_deviation_pct: 100.4,
          max_abs_deviation_pct: 110,
          direction: "UP",
          corroborates: true,
        },
        {
          metric: "power_factor",
          evaluated_readings: 58,
          triggered_readings: 50,
          median_observed: 0.75,
          median_baseline: 0.94,
          median_deviation_pct: -20.3,
          max_abs_deviation_pct: 24,
          direction: "DOWN",
          corroborates: true,
        },
        {
          metric: "voltage_v",
          evaluated_readings: 58,
          triggered_readings: 0,
          median_observed: 219,
          median_baseline: 220,
          median_deviation_pct: -0.4,
          max_abs_deviation_pct: 1.5,
          direction: null,
          corroborates: false,
        },
      ],
      related_events: [
        {
          timestamp: "2030-01-12T14:00:00",
          type: "UNKNOWN",
          description: "Nothing was reported <b>here</b>",
          offset_seconds: 0,
          role: "CONTEXT",
        },
      ],
      confidence_breakdown: {
        signal_strength: 1,
        persistence: 1,
        multivariate_support: 0.667,
        event_context: 1,
        pattern_support: 1,
      },
      signals: [
        {
          metric: "consumption_kwh",
          timestamp: "2030-01-12T14:00:00",
          observed: 90,
          baseline: 46,
          deviation: 44,
          deviation_pct: 95.6,
          robust_z: 18.4,
          direction: "UP",
          strength: 3.8,
        },
      ],
    },
    ...overrides,
  };
}

/** A stored explanation (fictional text); override source/model/fallback per test. */
export function explanation(overrides: Partial<Explanation> = {}): Explanation {
  return {
    source: "DETERMINISTIC",
    summary: "Consumption rose above its hourly baseline and no recorded event explains the change.",
    why_it_matters: "A persistent unexplained change may point to a fault or an unexpected load.",
    evidence_narrative: "Current and power factor changed in the same period; the unknown event is context only.",
    recommended_action_text: "Inspect the meter and its installation on site.",
    model: null,
    prompt_version: "evidence-template-v1",
    generated_at: "2030-02-01T10:00:02Z",
    fallback_used: false,
    ...overrides,
  };
}
