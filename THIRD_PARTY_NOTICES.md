# Сторонние компоненты и смоделированные интеграции

Документ перечисляет всё, что использовано в «Матрице» и создано не нами: библиотеки, образы, шрифты, фотографии, модель ИИ. Точные версии зафиксированы в `go.mod`/`go.sum`, `web/package-lock.json` и `docker-compose.yml`; полный список зависимостей с лицензиями формируется из этих файлов.

## Смоделированные интеграции

В продукте нет подключений к реальным системам. Следующее воспроизведено локально, данные синтетические.

| Что имитируется | Как | Где |
|---|---|---|
| Бюро кредитных историй (БКИ) | Сервис `mock-bki`: балл, активные кредиты, долг, максимальная просрочка по ИНН | `cmd/mock-bki`, `internal/mocks` |
| Единый государственный реестр юридических лиц и ИП (ЕГРЮЛ/ЕГРИП) | Сервис `mock-egrul`: статус, ОКВЭД, дата регистрации, признаки банкротства, дисквалификации руководителя и недостоверности сведений, отчётность и анализ выписки по счёту | `cmd/mock-egrul`, `internal/mocks` |
| Подписание договора и выдача денег | Переходы `SIGNING → DISBURSED` без реальных операций; интерфейс помечает их как демонстрационные | `internal/workflow`, `internal/pipeline` |
| Данные компаний, финансовые показатели, пользователи | Синтетический набор из 11 компаний-клиентов, кредитного аналитика и специалиста поддержки; для любого ИНН с верной контрольной суммой при регистрации синтезируется своя компания | `internal/seed` |

Сбой внешних источников можно воспроизвести: `POST http://localhost:8081/mock/fail?on=true` (бюро), `POST http://localhost:8082/mock/fail?on=true` (реестр).

## Модель ИИ (по желанию)

| Компонент | Лицензия | Примечание |
|---|---|---|
| Ollama (образ `ollama/ollama:0.30.10`) | MIT | Запускается только с профилем `ai` или на хосте; по умолчанию ИИ выключен |
| T-lite-it-2.1 (Т-Банк), квантование Q4_K_M (`hf.co/t-tech/T-lite-it-2.1-GGUF:Q4_K_M`) | Apache 2.0 | Открытые веса, 8 млрд параметров, основа — Qwen3-8B (Apache 2.0). Скачиваются скриптом `scripts/pull-model.sh` (около 5 ГБ), в репозиторий не входят. Подбирает продукт и пересказывает оценку; ответ проверяется кодом, решений не принимает. Выбрана по сравнению в [docs/ai-chat.md](docs/ai-chat.md) |
| Qwen3.5 4B (`qwen3.5:4b`) | Apache 2.0 | Только для сравнения моделей при разработке (`cmd/chatcompare`), в поставку не входит |

Платные API и закрытые модели не используются. Любой сбой модели переключает на правила.

## Серверная часть (Go)

| Компонент | Лицензия |
|---|---|
| github.com/go-chi/chi | MIT |
| github.com/jackc/pgx | MIT |
| github.com/pressly/goose | MIT |
| github.com/golang-jwt/jwt | MIT |
| github.com/google/uuid | BSD-3-Clause |
| golang.org/x/crypto | BSD-3-Clause |
| github.com/oapi-codegen/runtime | Apache-2.0 |
| github.com/getkin/kin-openapi (тесты контракта) | MIT |
| oapi-codegen, sqlc (только при разработке, генерация кода) | Apache-2.0, MIT |

Транзитивные зависимости перечислены в `go.sum`.

## Клиентская часть

| Компонент | Лицензия |
|---|---|
| React, React DOM | MIT |
| react-router | MIT |
| TanStack Query | MIT |
| Framer Motion (пакет `framer-motion`) | MIT |
| Tailwind CSS | MIT |
| Vite, плагин React для Vite | MIT |
| TypeScript | Apache-2.0 |
| Swagger UI (`swagger-ui-dist`, страница `/docs`) | Apache-2.0 |
| Vitest, Testing Library, jsdom | MIT |
| openapi-typescript, oxlint | MIT |
| Playwright (e2e) | Apache-2.0 |
| Шрифт Onest (`@fontsource-variable/onest`) | SIL Open Font License 1.1 |

## Образы Docker

| Образ | Назначение |
|---|---|
| `postgres:16.4-alpine` | База данных (лицензия PostgreSQL) |
| `golang:1.26-alpine` | Сборка Go-сервисов (BSD-3-Clause) |
| `alpine:3.20` | Базовый образ Go-сервисов |
| `node:22-alpine` | Сборка веб-приложения (MIT) |
| `nginx:1.27-alpine` | Раздача веб-приложения (BSD-2-Clause) |

## Фотографии на лендинге

Три фотографии из Wikimedia Commons, лицензия CC BY 2.0, автор Shixart1985. Указание авторства размещено внизу лендинга. Описание файлов и ссылки — в `web/public/photos/credits.json`.

| Файл | Название | Источник |
|---|---|---|
| `baker.jpg` | Baker woman's hands kneading bread dough | [Wikimedia Commons](https://commons.wikimedia.org/wiki/File:Baker_woman%27s_hands_kneading_bread_dough.jpg) |
| `barista.jpg` | Barista prepares espresso at coffee shop | [Wikimedia Commons](https://commons.wikimedia.org/wiki/File:Barista_prepares_espresso_at_coffee_shop.jpg) |
| `workshop.jpg` | Craftsman shaping wood in a workshop… | [Wikimedia Commons](https://commons.wikimedia.org/wiki/File:Craftsman_shaping_wood_in_a_workshop_during_daylight_hours_with_focus_on_precision_and_detail_in_woodworking.jpg) |

## Что не использовано

Закрытые и платные API, покупные наборы данных, корпоративные системы Альфа-Банка, официальный логотип, шрифт и торговые знаки банка. Стиль вдохновлён брендом из кейса, но знаков банка не воспроизводит.
