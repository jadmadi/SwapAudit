//go:build linux

package watch

import (
	"strconv"
	"time"

	"github.com/jadmadi/swapaudit/internal/live"
)

// Config controls danger evaluation and per-resource fatigue.
type Config struct {
	ETAThreshold      time.Duration
	PressureThreshold float64
	Sustain           time.Duration
	Cooldown          time.Duration
	MaxPerHour        int
	RearmETA          time.Duration
	RearmPressure     float64
	Resource          string
	NotifyRecovery    bool
	Interval          time.Duration
	TmpInterval       time.Duration
	Now               func() time.Time
}

// State is the in-memory per-resource state machine.
type State struct {
	Status        string
	DangerSince   time.Time
	Evaluations   int
	LastAlert     time.Time
	LastSeverity  string
	Escalated     bool
	Hourly        []time.Time
	IncidentKey   string
	RecoverySince time.Time
	CapNotified   time.Time
	StaleNotified bool
}

// TopGrower is the JSON shape of the largest positive mover.
type TopGrower struct {
	Kind             string `json:"kind"`
	PID              *int   `json:"pid"`
	StartTicks       *int64 `json:"start_ticks"`
	Name             string `json:"name"`
	DeltaBytes       int64  `json:"delta_bytes"`
	DeltaBytesPerMin int64  `json:"delta_bytes_per_min"`
	CurrentBytes     int64  `json:"current_bytes"`
}

// Event is the JSON line emitted in --json mode.
type Event struct {
	Type                  string     `json:"type"`
	Resource              string     `json:"resource,omitempty"`
	Severity              string     `json:"severity,omitempty"`
	SampledAt             string     `json:"sampled_at"`
	ETASeconds            *int64     `json:"eta_seconds"`
	PressurePct           float64    `json:"pressure_pct,omitempty"`
	RateBytesPerSec       float64    `json:"rate_bytes_per_sec,omitempty"`
	Top                   *TopGrower `json:"top"`
	ReclaimableBytes      int64      `json:"reclaimable_bytes,omitempty"`
	ReclaimableAgeSeconds int64      `json:"reclaimable_age_seconds,omitempty"`
	Transport             *string    `json:"transport"`
	Message               string     `json:"message,omitempty"`
	Reason                string     `json:"reason,omitempty"`
}

// Decision is one notification or JSON event.
type Decision struct {
	Kind     string
	Reason   string
	Resource string
	Severity string
	Title    string
	Body     string
	Event    Event
}

// Evaluate mutates the per-resource state and returns at most one decision per
// resource, in swap-before-tmpfs order.
func Evaluate(cfg Config, prior map[string]*State, stats live.WindowStats) []Decision {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	now := cfg.Now()
	resources := []string{"swap", "tmpfs"}
	switch cfg.Resource {
	case "swap":
		resources = []string{"swap"}
	case "tmpfs":
		resources = []string{"tmpfs"}
	}
	var out []Decision
	for _, resource := range resources {
		st := prior[resource]
		if st == nil {
			st = &State{Status: "armed"}
			prior[resource] = st
		}
		if d, ok := evaluateResource(cfg, st, stats, resource, now); ok {
			out = append(out, d)
		}
	}
	return out
}

func evaluateResource(cfg Config, st *State, stats live.WindowStats, resource string, now time.Time) (Decision, bool) {
	series, used, total := metrics(stats, resource)
	pct := percent(used, total)

	if resource == "tmpfs" && stats.TmpfsSampledAgeSec > int64((2*cfg.TmpInterval).Seconds()) {
		if st.StaleNotified {
			return Decision{}, false
		}
		st.StaleNotified = true
		return suppressed(cfg, stats, resource, now, "stale_tmpfs"), true
	}
	st.StaleNotified = false

	if stats.SampledAt.IsZero() {
		stats.SampledAt = now
	}
	eta := series.ETASeconds
	rising := series.RateBytesPerSec > 0
	etaDanger := eta != nil && *eta <= int64(cfg.ETAThreshold.Seconds())
	pressureDanger := pct >= cfg.PressureThreshold && rising
	danger := etaDanger || pressureDanger
	critical := (eta != nil && *eta <= int64(cfg.ETAThreshold.Seconds())/3) || pct >= 95
	clear := !((eta != nil && *eta <= int64(cfg.RearmETA.Seconds())) || (pct >= cfg.RearmPressure && rising))

	switch st.Status {
	case "", "armed":
		if danger {
			st.Status, st.DangerSince, st.Evaluations = "pending", now, 1
		}
	case "pending":
		if !danger {
			st.Status, st.DangerSince, st.Evaluations = "armed", time.Time{}, 0
			return Decision{}, false
		}
		st.Evaluations++
		if now.Sub(st.DangerSince) < cfg.Sustain || st.Evaluations < 2 {
			return Decision{}, false
		}
		return alert(cfg, st, stats, resource, now, pct, critical)
	case "alerted":
		if clear {
			if st.RecoverySince.IsZero() {
				st.RecoverySince = now
			} else if now.Sub(st.RecoverySince) >= cfg.Sustain {
				st.Status, st.RecoverySince = "recovering", now
			}
		} else {
			st.RecoverySince = time.Time{}
		}
		if danger && critical && st.LastSeverity == "warning" && !st.Escalated {
			st.Escalated = true
			if len(pruneHourly(st, now)) >= cfg.MaxPerHour {
				return suppressed(cfg, stats, resource, now, "hourly_cap"), true
			}
			st.Status, st.LastSeverity, st.LastAlert = "alerted", "critical", now
			st.Hourly = append(st.Hourly, now)
			top := TopMover(stats, resource)
			sustained := int64(now.Sub(st.DangerSince).Seconds())
			stats.SustainedSeconds = sustained
			title := FormatTitle(resource, "critical", eta, pct)
			body := FormatBody(resource, stats, top, reclaimable(stats, resource), reclaimAge(stats, resource))
			return Decision{Kind: "alert", Resource: resource, Severity: "critical", Title: title, Body: body,
				Event: buildEvent("alert", resource, "critical", stats, top, now, title, "")}, true
		}
	case "recovering":
		if danger {
			st.Status, st.DangerSince, st.Evaluations, st.RecoverySince = "pending", now, 1, time.Time{}
			return Decision{}, false
		}
		if !clear {
			st.RecoverySince = time.Time{}
			return Decision{}, false
		}
		if st.RecoverySince.IsZero() {
			st.RecoverySince = now
			return Decision{}, false
		}
		if now.Sub(st.RecoverySince) >= cfg.Sustain {
			st.Status = "armed"
			if cfg.NotifyRecovery {
				title := "swapaudit: " + resource + " recovered"
				body := FormatBody(resource, stats, TopMover(stats, resource), reclaimable(stats, resource), reclaimAge(stats, resource))
				st.LastSeverity, st.IncidentKey = "", ""
				return Decision{Kind: "recovered", Resource: resource, Title: title, Body: body,
					Event: buildEvent("recovered", resource, "", stats, nil, now, "recovered", "")}, true
			}
		}
	}
	return Decision{}, false
}

func alert(cfg Config, st *State, stats live.WindowStats, resource string, now time.Time, pct float64, critical bool) (Decision, bool) {
	severity := "warning"
	if critical {
		severity = "critical"
	}
	top := TopMover(stats, resource)
	key := dedupKey(resource, top, severity)
	st.Hourly = pruneHourly(st, now)

	if len(st.Hourly) >= cfg.MaxPerHour {
		st.Status = "alerted"
		if st.CapNotified.IsZero() || now.Sub(st.CapNotified) >= time.Hour {
			st.CapNotified = now
			return suppressed(cfg, stats, resource, now, "hourly_cap"), true
		}
		return Decision{}, false
	}
	if !st.LastAlert.IsZero() && now.Sub(st.LastAlert) < cfg.Cooldown {
		st.Status = "alerted"
		return suppressed(cfg, stats, resource, now, "cooldown"), true
	}
	dedupWindow := cfg.Cooldown
	if dedupWindow <= 0 {
		dedupWindow = cfg.Sustain
	}
	if st.IncidentKey == key && !st.LastAlert.IsZero() && now.Sub(st.LastAlert) < dedupWindow {
		st.Status = "alerted"
		return suppressed(cfg, stats, resource, now, "dedup"), true
	}

	st.Status, st.LastAlert, st.LastSeverity, st.Escalated = "alerted", now, severity, critical
	st.IncidentKey = key
	st.Hourly = append(st.Hourly, now)
	stats.SustainedSeconds = int64(now.Sub(st.DangerSince).Seconds())
	series, _, _ := metrics(stats, resource)
	title := FormatTitle(resource, severity, series.ETASeconds, pct)
	body := FormatBody(resource, stats, top, reclaimable(stats, resource), reclaimAge(stats, resource))
	return Decision{Kind: "alert", Resource: resource, Severity: severity, Title: title, Body: body,
		Event: buildEvent("alert", resource, severity, stats, top, now, title, "")}, true
}

func suppressed(cfg Config, stats live.WindowStats, resource string, now time.Time, reason string) Decision {
	return Decision{Kind: "suppressed", Reason: reason, Resource: resource,
		Event: buildEvent("suppressed", resource, "", stats, nil, now, "", reason)}
}

func metrics(stats live.WindowStats, resource string) (live.SeriesStats, int64, int64) {
	if resource == "tmpfs" {
		return stats.Tmpfs, stats.System.TmpfsUsed, stats.System.TmpfsSize
	}
	return stats.Swap, stats.System.SwapUsed, stats.System.SwapTotal
}

func percent(used, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(used) / float64(total) * 100
}

func reclaimable(stats live.WindowStats, resource string) int64 {
	if resource == "tmpfs" {
		return stats.TmpfsCleanableBytes
	}
	return 0
}

func reclaimAge(stats live.WindowStats, resource string) time.Duration {
	if resource == "tmpfs" {
		return time.Duration(stats.TmpfsSampledAgeSec) * time.Second
	}
	return 0
}

func pruneHourly(st *State, now time.Time) []time.Time {
	kept := st.Hourly[:0]
	for _, t := range st.Hourly {
		if now.Sub(t) < time.Hour {
			kept = append(kept, t)
		}
	}
	st.Hourly = kept
	return kept
}

func dedupKey(resource string, top *live.Mover, severity string) string {
	if top == nil {
		return resource + "|none|" + severity
	}
	if top.Kind == "cache" {
		return resource + "|cache:" + top.Name + "|" + severity
	}
	pid := 0
	if top.PID != nil {
		pid = *top.PID
	}
	ticks := int64(0)
	if top.StartTicks != nil {
		ticks = *top.StartTicks
	}
	return resource + "|process:" + strconv.Itoa(pid) + ":" + strconv.FormatInt(ticks, 10) + "|" + severity
}

// TopMover returns the largest positive, non-exited mover for a resource.
func TopMover(stats live.WindowStats, resource string) *live.Mover {
	movers := stats.Movers
	if resource == "tmpfs" {
		movers = stats.TmpfsMovers
	}
	for i := range movers {
		if !movers[i].Exited && movers[i].DeltaBytes > 0 {
			return &movers[i]
		}
	}
	return nil
}

func buildEvent(kind, resource, severity string, stats live.WindowStats, top *live.Mover, now time.Time, message, reason string) Event {
	series, used, total := metrics(stats, resource)
	ev := Event{
		Type: kind, Resource: resource, Severity: severity, SampledAt: now.Format(time.RFC3339),
		ETASeconds: series.ETASeconds, PressurePct: percent(used, total), RateBytesPerSec: series.RateBytesPerSec,
		ReclaimableBytes: reclaimable(stats, resource), ReclaimableAgeSeconds: int64(reclaimAge(stats, resource) / time.Second),
		Message: message, Reason: reason,
	}
	if top != nil {
		tg := &TopGrower{
			Kind: top.Kind, PID: top.PID, StartTicks: top.StartTicks, Name: top.Name,
			DeltaBytes: top.DeltaBytes, CurrentBytes: top.CurrentBytes,
		}
		if stats.Seconds > 0 {
			tg.DeltaBytesPerMin = top.DeltaBytes * 60 / int64(stats.Seconds)
		}
		ev.Top = tg
	}
	return ev
}
