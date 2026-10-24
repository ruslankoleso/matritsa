// Package seed holds the synthetic demo data shared by the seeder and the
// BKI/EGRUL mocks. Everything here is invented; INNs do not belong to real entities.
package seed

import (
	"time"

	"matritsa/internal/integrations"
)

const rub = int64(100)

// DemoPassword is the password of all demo accounts (demo data only).
const DemoPassword = "demo1234"

type Company struct {
	Login              string // client login
	OwnerName          string
	INN                string
	Name               string
	Kind               string // ooo | ip
	RegisteredAt       time.Time
	OKVED              string
	OKVEDTitle         string
	OKVEDGroup         int
	Director           string
	Address            string
	MonthlyObligations int64
	Revenue            []int64 // monthly, oldest first, ends at last full month
	Status             string  // EGRUL status
	Bankrupt           bool
	BKI                *integrations.BKIReport // nil: company unknown to the bureau
	Story              string                  // what this demo company illustrates
}

func monthsAgo(n int) time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month()-time.Month(n), 1, 0, 0, 0, 0, time.UTC)
}

// series builds monthly revenue from a base and a repeating relative pattern.
func series(base int64, pattern []float64, n int) []int64 {
	out := make([]int64, n)
	for i := range out {
		out[i] = int64(float64(base) * pattern[i%len(pattern)])
	}
	return out
}

// Companies returns the demo companies: each one tells a different story.
func Companies() []Company {
	return []Company{
		{
			Login: "client1", OwnerName: "Ирина Волкова", INN: "7701234501", Name: "ООО «Тёплый хлеб»", Kind: "ooo",
			RegisteredAt: monthsAgo(72), OKVED: "10.71", OKVEDTitle: "Производство хлеба и мучных кондитерских изделий", OKVEDGroup: 1,
			Director: "Волкова Ирина Сергеевна", Address: "г. Казань, ул. Баумана, 14",
			MonthlyObligations: 80_000 * rub,
			Revenue:            series(2_400_000*rub, []float64{1.0, 0.97, 1.03, 1.02, 0.98, 1.05}, 12),
			Status:             "active",
			BKI:                &integrations.BKIReport{HasHistory: true, ActiveLoans: 1, TotalDebt: 1_200_000 * rub, MaxOverdueDays: 0, Score: 790, ClosedLoans: 3, TotalRepaid: 3_200_000 * rub},
			Story:              "Надёжная пекарня: решение автоматически, без сотрудника",
		},
		{
			Login: "client2", OwnerName: "Алексей Смирнов", INN: "771234500002", Name: "ИП Смирнов А. В. (кофейня «Гараж»)", Kind: "ip",
			RegisteredAt: monthsAgo(96), OKVED: "56.10", OKVEDTitle: "Деятельность ресторанов и кафе", OKVEDGroup: 2,
			Director: "Смирнов Алексей Викторович", Address: "г. Нижний Новгород, ул. Рождественская, 7",
			MonthlyObligations: 120_000 * rub,
			Revenue:            series(700_000*rub, []float64{1.0, 0.8, 1.15, 0.9, 1.2, 0.85}, 12),
			Status:             "active",
			BKI:                &integrations.BKIReport{HasHistory: true, ActiveLoans: 3, TotalDebt: 3_400_000 * rub, MaxOverdueDays: 25, Score: 610, ClosedLoans: 2, TotalRepaid: 1_500_000 * rub},
			Story:              "Закредитованный ИП: сотрудник предлагает меньшую сумму (встречное предложение)",
		},
		{
			Login: "client3", OwnerName: "Мария Орлова", INN: "7701234503", Name: "ООО «Новый старт»", Kind: "ooo",
			RegisteredAt: monthsAgo(4), OKVED: "62.01", OKVEDTitle: "Разработка программного обеспечения", OKVEDGroup: 1,
			Director: "Орлова Мария Андреевна", Address: "г. Москва, Ленинградский пр-т, 39",
			MonthlyObligations: 0,
			Revenue:            []int64{260_000 * rub, 340_000 * rub, 380_000 * rub, 430_000 * rub},
			Status:             "active",
			BKI:                &integrations.BKIReport{HasHistory: false},
			Story:              "Молодая компания без кредитной истории: заявку смотрит сотрудник",
		},
		{
			Login: "client4", OwnerName: "Пётр Горин", INN: "7701234504", Name: "ООО «Старый порт»", Kind: "ooo",
			RegisteredAt: monthsAgo(120), OKVED: "52.10", OKVEDTitle: "Складское хранение", OKVEDGroup: 2,
			Director: "Горин Пётр Николаевич", Address: "г. Санкт-Петербург, наб. Обводного канала, 118",
			MonthlyObligations: 0,
			Revenue:            series(700_000*rub, []float64{1.0, 0.9, 1.1}, 12),
			Status:             "liquidated",
			BKI:                &integrations.BKIReport{HasHistory: true, ActiveLoans: 0, Score: 700, ClosedLoans: 2, TotalRepaid: 1_800_000 * rub},
			Story:              "Ликвидированная компания: автоматический отказ с понятной причиной",
		},
		{
			Login: "client5", OwnerName: "Елена Кузнецова", INN: "771234500005", Name: "ИП Кузнецова Е. С. (салон «Лотос»)", Kind: "ip",
			RegisteredAt: monthsAgo(40), OKVED: "96.02", OKVEDTitle: "Предоставление услуг парикмахерскими и салонами красоты", OKVEDGroup: 2,
			Director: "Кузнецова Елена Сергеевна", Address: "г. Екатеринбург, ул. Малышева, 51",
			MonthlyObligations: 60_000 * rub,
			Revenue:            series(600_000*rub, []float64{1.0, 0.92, 1.08, 1.04, 0.95}, 12),
			Status:             "active",
			BKI:                &integrations.BKIReport{HasHistory: true, ActiveLoans: 1, TotalDebt: 500_000 * rub, MaxOverdueDays: 20, Score: 620, ClosedLoans: 1, TotalRepaid: 800_000 * rub},
			Story:              "Пограничный случай: решение принимает сотрудник",
		},
		{
			Login: "client6", OwnerName: "Игорь Тихонов", INN: "7701234506", Name: "ООО «ТехноСтрой»", Kind: "ooo",
			RegisteredAt: monthsAgo(60), OKVED: "41.20", OKVEDTitle: "Строительство жилых и нежилых зданий", OKVEDGroup: 3,
			Director: "Тихонов Игорь Павлович", Address: "г. Новосибирск, Красный пр-т, 82",
			MonthlyObligations: 200_000 * rub,
			Revenue:            series(1_500_000*rub, []float64{1.0, 0.7, 1.3, 0.9}, 12),
			Status:             "active",
			BKI:                &integrations.BKIReport{HasHistory: true, ActiveLoans: 4, TotalDebt: 9_000_000 * rub, MaxOverdueDays: 120, Score: 480, ClosedLoans: 1, TotalRepaid: 2_000_000 * rub},
			Story:              "Просрочка более 90 дней: отказ по жёсткому правилу",
		},
		{
			Login: "client7", OwnerName: "Андрей Лебедев", INN: "7701234507", Name: "ООО «Балтийский лён»", Kind: "ooo",
			RegisteredAt: monthsAgo(90), OKVED: "13.20", OKVEDTitle: "Производство тканей", OKVEDGroup: 1,
			Director: "Лебедев Андрей Игоревич", Address: "г. Калининград, ул. Профсоюзная, 33",
			MonthlyObligations: 150_000 * rub,
			Revenue:            series(4_000_000*rub, []float64{1.0, 0.97, 1.03, 1.02, 0.98, 1.05}, 12),
			Status:             "active",
			BKI:                &integrations.BKIReport{HasHistory: true, ActiveLoans: 1, TotalDebt: 2_000_000 * rub, MaxOverdueDays: 0, Score: 800, ClosedLoans: 5, TotalRepaid: 12_000_000 * rub},
			Story:              "Надёжная компания, но сумма выше лимита автоматического решения: рассматривает сотрудник",
		},
		{
			Login: "client8", OwnerName: "Константин Орлов", INN: "771234500008", Name: "ИП Орлов К. А. (мастерская «Дуб»)", Kind: "ip",
			RegisteredAt: monthsAgo(60), OKVED: "16.23", OKVEDTitle: "Производство прочих деревянных строительных конструкций", OKVEDGroup: 1,
			Director: "Орлов Константин Александрович", Address: "г. Тверь, ул. Советская, 21",
			MonthlyObligations: 60_000 * rub,
			Revenue:            series(1_200_000*rub, []float64{1.0, 0.97, 1.03, 1.02, 0.98, 1.05}, 12),
			Status:             "active",
			BKI:                &integrations.BKIReport{HasHistory: true, ActiveLoans: 1, TotalDebt: 500_000 * rub, MaxOverdueDays: 0, Score: 770, ClosedLoans: 1, TotalRepaid: 360_000 * rub},
			Story:              "Хороший заёмщик с малым опытом погашения: автоматического решения нет, рассматривает сотрудник",
		},
		{
			Login: "client9", OwnerName: "Виктор Зуев", INN: "7701234509", Name: "ООО «Вектор Логистик»", Kind: "ooo",
			RegisteredAt: monthsAgo(48), OKVED: "49.41", OKVEDTitle: "Деятельность автомобильного грузового транспорта", OKVEDGroup: 3,
			Director: "Зуев Виктор Петрович", Address: "г. Самара, Московское шоссе, 17",
			MonthlyObligations: 140_000 * rub,
			Revenue:            series(500_000*rub, []float64{1.0, 0.9, 1.1, 0.95, 1.05}, 12),
			Status:             "active",
			BKI:                &integrations.BKIReport{HasHistory: true, ActiveLoans: 4, TotalDebt: 6_000_000 * rub, MaxOverdueDays: 15, Score: 600, ClosedLoans: 1, TotalRepaid: 1_000_000 * rub},
			Story:              "Платежи по действующим кредитам съедают выручку: автоматический отказ по платёжеспособности",
		},
		{
			Login: "client10", OwnerName: "Сергей Мартынов", INN: "7701234510", Name: "ООО «Гранд Строй»", Kind: "ooo",
			RegisteredAt: monthsAgo(84), OKVED: "41.20", OKVEDTitle: "Строительство жилых и нежилых зданий", OKVEDGroup: 3,
			Director: "Мартынов Сергей Олегович", Address: "г. Краснодар, ул. Красная, 120",
			MonthlyObligations: 100_000 * rub, Bankrupt: true,
			Revenue: series(2_000_000*rub, []float64{1.0, 0.8, 1.2, 0.9}, 12),
			Status:  "active",
			BKI:     &integrations.BKIReport{HasHistory: true, ActiveLoans: 2, TotalDebt: 4_000_000 * rub, MaxOverdueDays: 40, Score: 560, ClosedLoans: 2, TotalRepaid: 3_000_000 * rub},
			Story:   "Процедура банкротства: автоматический отказ по жёсткому правилу",
		},
		{
			Login: "client11", OwnerName: "Марина Васильева", INN: "771234500011", Name: "ИП Васильева М. Н. (студия «Лад»)", Kind: "ip",
			RegisteredAt: monthsAgo(54), OKVED: "96.02", OKVEDTitle: "Предоставление услуг парикмахерскими и салонами красоты", OKVEDGroup: 2,
			Director: "Васильева Марина Николаевна", Address: "г. Ярославль, ул. Свободы, 9",
			MonthlyObligations: 40_000 * rub,
			Revenue:            series(900_000*rub, []float64{1.0, 0.97, 1.03, 1.02, 0.98, 1.05}, 12),
			Status:             "active",
			BKI:                &integrations.BKIReport{HasHistory: true, ActiveLoans: 0, TotalDebt: 0, MaxOverdueDays: 0, Score: 760, ClosedLoans: 4, TotalRepaid: 5_000_000 * rub},
			Story:              "Постоянный клиент: полная история заявок — выдача, отказ от предложения, черновик и отозванная заявка",
		},
	}
}

// ByINN finds a demo company.
func ByINN(inn string) (Company, bool) {
	for _, c := range Companies() {
		if c.INN == inn {
			return c, true
		}
	}
	return Company{}, false
}

// EGRUL builds the registry record for a demo company.
func (c Company) EGRUL() integrations.EGRULRecord {
	return integrations.EGRULRecord{
		INN: c.INN, Name: c.Name, Status: c.Status, RegisteredAt: c.RegisteredAt,
		OKVED: c.OKVED, Director: c.Director, Address: c.Address, Bankrupt: c.Bankrupt,
	}
}

// Staff accounts.
type Staff struct{ Login, FullName string }

func StaffAccounts() []Staff {
	return []Staff{{Login: "staff1", FullName: "Анна Сергеевна Белова, кредитный аналитик"}}
}
