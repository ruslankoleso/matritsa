package decision

import (
	"strings"

	"matritsa/internal/scoring"
)

// NextSteps says what the client can do after a refusal with these reasons and whether a new
// application makes sense right away. A hard stop is about the company itself: a smaller amount or
// another product would be refused the same way, so the client is told what has to change first.
func NextSteps(reasons []string) (advice string, canReapply bool) {
	has := func(want string) bool {
		for _, r := range reasons {
			if r == want || strings.HasPrefix(r, want) {
				return true
			}
		}
		return false
	}
	switch {
	case has(scoring.StopInactive), has(scoring.StopBankrupt):
		return "Банк не выдаёт кредиты компании, которая прекратила деятельность или находится в процедуре банкротства. Новая заявка от этой компании будет отклонена по той же причине.", false
	case has(scoring.StopDisqualified):
		return "Подать заявку снова можно после смены руководителя и внесения изменений в реестр.", false
	case has(scoring.StopUnreliable):
		return "Обновите сведения о компании в реестре (ЕГРЮЛ/ЕГРИП). Когда отметка о недостоверности будет снята, заявку можно подать снова.", false
	case has(scoring.StopOverduePrefix):
		return "Погасите просроченную задолженность. Когда бюро кредитных историй обновит данные, заявку можно подать снова.", false
	case has(scoring.StopNotFound):
		return "Банк не нашёл компанию в реестре. Проверьте ИНН и подайте заявку снова.", true
	case has(scoring.StopNoRevenue):
		return "Банку нужно видеть поступления на расчётный счёт. Подайте новую заявку и приложите выписку с оборотами за последние 12 месяцев.", true
	}
	return "Можно подать заявку на меньшую сумму или на более долгий срок: платёж станет меньше. Если выручка вырастет или долговая нагрузка снизится, подайте заявку снова.", true
}
