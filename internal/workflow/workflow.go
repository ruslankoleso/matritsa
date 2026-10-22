// Package workflow is the pure state machine of a credit application.
// It knows nothing about storage or HTTP: only which transitions are legal.
package workflow

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type Status string

const (
	Draft            Status = "DRAFT"
	Submitted        Status = "SUBMITTED"
	IntakeCheck      Status = "INTAKE_CHECK"
	NeedsInfo        Status = "NEEDS_INFO"
	Enrichment       Status = "ENRICHMENT"
	Scoring          Status = "SCORING"
	AnalystReview    Status = "ANALYST_REVIEW"
	OfferMade        Status = "OFFER_MADE"
	Rejected         Status = "REJECTED"
	Signing          Status = "SIGNING"
	Disbursed        Status = "DISBURSED"
	DeclinedByClient Status = "DECLINED_BY_CLIENT"
	Expired          Status = "EXPIRED"
	Cancelled        Status = "CANCELLED"
)

type Event string

const (
	Submit             Event = "Submit"
	StartIntake        Event = "StartIntake"
	IntakePassed       Event = "IntakePassed"
	IntakeNeedsInfo    Event = "IntakeNeedsInfo"
	ClientProvidedInfo Event = "ClientProvidedInfo"
	ClientAnswered     Event = "ClientAnswered"
	EnrichmentDone     Event = "EnrichmentDone"
	EnrichmentFailed   Event = "EnrichmentFailed"
	AutoOffer          Event = "AutoOffer"
	SendToReview       Event = "SendToReview"
	AutoReject         Event = "AutoReject"
	AnalystOffer       Event = "AnalystOffer"
	AnalystRequestInfo Event = "AnalystRequestInfo"
	AnalystReject      Event = "AnalystReject"
	ClientAccept       Event = "ClientAccept"
	ClientDecline      Event = "ClientDecline"
	OfferExpired       Event = "OfferExpired"
	Disburse           Event = "Disburse"
	Cancel             Event = "Cancel"
)

var ErrInvalidTransition = errors.New("invalid state transition")

var transitions = map[Status]map[Event]Status{
	Draft:         {Submit: Submitted, Cancel: Cancelled},
	Submitted:     {StartIntake: IntakeCheck},
	IntakeCheck:   {IntakePassed: Enrichment, IntakeNeedsInfo: NeedsInfo},
	NeedsInfo:     {ClientProvidedInfo: Enrichment, ClientAnswered: AnalystReview, Cancel: Cancelled},
	Enrichment:    {EnrichmentDone: Scoring, EnrichmentFailed: AnalystReview},
	Scoring:       {AutoOffer: OfferMade, SendToReview: AnalystReview, AutoReject: Rejected},
	AnalystReview: {AnalystOffer: OfferMade, AnalystRequestInfo: NeedsInfo, AnalystReject: Rejected, Cancel: Cancelled},
	OfferMade:     {ClientAccept: Signing, ClientDecline: DeclinedByClient, OfferExpired: Expired, Cancel: Cancelled},
	Signing:       {Disburse: Disbursed},
}

var order = []Status{Draft, Submitted, IntakeCheck, NeedsInfo, Enrichment, Scoring, AnalystReview, OfferMade, Signing, Disbursed, Rejected, DeclinedByClient, Expired, Cancelled}

// AllStatuses lists every status in process order.
func AllStatuses() []Status { return append([]Status(nil), order...) }

// Next returns the status after ev, or ErrInvalidTransition.
func Next(from Status, ev Event) (Status, error) {
	if to, ok := transitions[from][ev]; ok {
		return to, nil
	}
	return from, fmt.Errorf("%w: %s from %s", ErrInvalidTransition, ev, from)
}

// Allowed lists events legal in the status, sorted for stable output.
func Allowed(from Status) []Event {
	evs := make([]Event, 0, len(transitions[from]))
	for ev := range transitions[from] {
		evs = append(evs, ev)
	}
	sort.Slice(evs, func(i, j int) bool { return evs[i] < evs[j] })
	return evs
}

func IsTerminal(s Status) bool { return len(transitions[s]) == 0 }

// ClientMessage explains in plain language what is going on in the status.
func ClientMessage(s Status) string {
	switch s {
	case Draft:
		return "Заявка сохранена как черновик. Дозаполните её и отправьте, когда будете готовы."
	case Submitted:
		return "Заявка отправлена, банк принял её в работу."
	case IntakeCheck:
		return "Проверяем, что в заявке есть все нужные сведения и документы."
	case NeedsInfo:
		return "Банку нужно уточнение. Посмотрите, что именно, и ответьте — это быстро."
	case Enrichment:
		return "Сверяем сведения о компании с государственным реестром и бюро кредитных историй."
	case Scoring:
		return "Оцениваем надёжность и считаем условия — обычно это занимает пару минут."
	case AnalystReview:
		return "Заявку рассматривает сотрудник банка. Как только решение будет готово, вы увидите его здесь."
	case OfferMade:
		return "Для вас готово предложение. Посмотрите условия и примите решение."
	case Rejected:
		return "К сожалению, банк не смог одобрить заявку. Ниже — причины и что можно сделать дальше."
	case Signing:
		return "Остался последний шаг — подписать документы. В демо подписание условное."
	case Disbursed:
		return "Готово: деньги отправлены на счёт (в демо — условно)."
	case DeclinedByClient:
		return "Вы отказались от предложения. Можно подать новую заявку в любой момент."
	case Expired:
		return "Срок действия предложения истёк. Можно подать новую заявку."
	case Cancelled:
		return "Заявка отозвана."
	}
	return ""
}

// DiagramMarkdown renders the transition table as a Mermaid state diagram.
func DiagramMarkdown() string {
	var b strings.Builder
	b.WriteString("# Диаграмма состояний заявки\n\nФайл сгенерирован тестом `internal/workflow` (`UPDATE_DOCS=1 go test ./internal/workflow`). Не редактировать вручную.\n\n```mermaid\nstateDiagram-v2\n    [*] --> DRAFT\n")
	for _, from := range order {
		evs := Allowed(from)
		for _, ev := range evs {
			to := transitions[from][ev]
			fmt.Fprintf(&b, "    %s --> %s: %s\n", from, to, ev)
		}
	}
	for _, s := range order {
		if IsTerminal(s) {
			fmt.Fprintf(&b, "    %s --> [*]\n", s)
		}
	}
	b.WriteString("```\n")
	return b.String()
}
