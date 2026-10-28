# hihihaton

Проект хакатона «Северный код»: подбор оборудования и планирование восстановления арктической станции.

Репозиторий подготовлен для разработки в облачной среде Codex.

## Запуск всего проекта

Нужны Git, Docker и Docker Compose v2. Из терминала:

```bash
git clone https://github.com/foryoucode/hihihaton.git
cd hihihaton
docker compose up --build -d --wait
```

Откройте `http://localhost:8080`. Compose соберёт интерфейс, Go-backend и Python-нейросеть. Миграции Goose применятся автоматически. Обученная модель уже включена, ключи API не нужны.

```bash
docker compose logs -f
docker compose down
```

История сохраняется в Docker-volume и переживает остановку. Команда `docker compose down -v` удалит её. Для другого порта: `PORT=8081 docker compose up --build -d --wait`.

Примеры стиля в `solution` и `client` отдельно запускать не нужно. Рабочий проект — в [north-code](north-code/README.md).
