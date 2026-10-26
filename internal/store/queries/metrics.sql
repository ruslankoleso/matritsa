-- Process quality measures from docs/product.md, counted from the audit trail.

-- name: DecisionMetrics :one
-- The first decision on each submitted application: by the rules (auto) or by an analyst,
-- and how long it took from submission.
WITH sub AS (
    SELECT application_id, min(at) AS submitted_at
    FROM application_events WHERE event = 'Submit' GROUP BY application_id
), dec AS (
    SELECT DISTINCT ON (application_id) application_id, event, at
    FROM application_events
    WHERE event IN ('AutoOffer', 'AutoReject', 'AnalystOffer', 'AnalystReject')
    ORDER BY application_id, at
), timed AS (
    SELECT dec.event IN ('AutoOffer', 'AutoReject') AS auto,
           dec.event IN ('AutoOffer', 'AnalystOffer') AS offered,
           extract(epoch FROM dec.at - sub.submitted_at)::float8 AS seconds
    FROM dec JOIN sub USING (application_id)
)
SELECT
    (SELECT count(*) FROM sub)::int AS submitted,
    count(*)::int AS decided,
    (count(*) FILTER (WHERE auto))::int AS decided_auto,
    (count(*) FILTER (WHERE offered))::int AS offered,
    coalesce(percentile_cont(0.5) WITHIN GROUP (ORDER BY seconds) FILTER (WHERE auto), 0)::float8 AS median_auto_seconds,
    coalesce(percentile_cont(0.5) WITHIN GROUP (ORDER BY seconds) FILTER (WHERE NOT auto), 0)::float8 AS median_analyst_seconds
FROM timed;

-- name: FollowUpMetrics :one
-- Returns for clarification, and what clients did with the offers they got.
SELECT
    (SELECT count(DISTINCT application_id) FROM application_events WHERE event = 'Submit')::int AS submitted,
    (SELECT count(DISTINCT application_id) FROM application_events WHERE event IN ('IntakeNeedsInfo', 'AnalystRequestInfo'))::int AS asked_info,
    (SELECT count(DISTINCT application_id) FROM application_events WHERE event = 'ClientAccept')::int AS accepted,
    (SELECT count(DISTINCT application_id) FROM application_events WHERE event = 'ClientDecline')::int AS declined,
    (SELECT count(DISTINCT application_id) FROM application_events WHERE event = 'OfferExpired')::int AS expired,
    (SELECT count(*) FROM applications WHERE status = 'ANALYST_REVIEW')::int AS waiting_analyst;
