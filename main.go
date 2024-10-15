package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Task представляет структуру задачи
type Task struct {
	ID      string `json:"id"`
	Date    string `json:"date"`
	Title   string `json:"title"`
	Comment string `json:"comment"`
	Repeat  string `json:"repeat"`
}

// DbRep представляет репозиторий базы данных
var db *sql.DB

// createDB инициализирует базу данных SQLite и создает таблицу.
func createDB() {
	db, err := sql.Open("sqlite3", "./scheduler.db")
	if err != nil {
		return
	}
	defer db.Close()

	commands := []string{
		`CREATE TABLE IF NOT EXISTS scheduler (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date CHAR(8),
			title TEXT,
			comment TEXT,
			repeat CHAR (128)
					)`,
		"CREATE INDEX IF NOT EXISTS indexdate ON scheduler (date)",
	}
	for _, cmd := range commands {
		if _, err := db.Exec(cmd); err != nil {
			return
		}
	}
}

// nextDateHandler обрабатывает HTTP-запрос для получения следующей даты задачи.
func nextDateHandler(w http.ResponseWriter, r *http.Request) {
	// Получает параметры "now", "date" и "repeat" из URL запроса.
	nowStr := r.URL.Query().Get("now")
	date := r.URL.Query().Get("date")
	repeat := r.URL.Query().Get("repeat")

	// Проверяет, что все необходимые параметры присутствуют.
	if nowStr == "" || date == "" || repeat == "" {
		http.Error(w, "Отсутствуют необходимые параметры", http.StatusBadRequest)
		return
	}

	// Преобразует параметр "now" в формат времени.
	now, err := time.Parse("20060102", nowStr)
	if err != nil {
		// Если формат некорректен, возвращает ошибку клиенту.
		http.Error(w, "Некорректный формат времени", http.StatusBadRequest)
		return
	}

	// Вызывает функцию для вычисления следующей даты.
	nextDate, err := NextDate(now, date, repeat)
	if err != nil {
		// Если возникает ошибка, возвращает ее клиенту.
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Возвращаем только следующую дату
	fmt.Fprintln(w, nextDate)
}

// NextDate вычисляет следующую дату задачи.
func NextDate(now time.Time, date string, repeat string) (string, error) {
	// Нормализуем 'now' до полуночи
	nowDateStr := now.Format("20060102")
	now, err := time.Parse("20060102", nowDateStr)
	if err != nil {
		return "", err
	}

	if repeat == "" {
		return "", errors.New("правило повторения отсутствует")
	}

	rep := strings.Split(repeat, " ")

	if len(rep) < 1 {
		return "", errors.New("некорректное правило повторения")
	}

	timBase, err := time.Parse("20060102", date)
	if err != nil {
		return "", err
	}

	if rep[0] == "y" {
		// Извлекаем день и месяц исходной даты
		origDay := timBase.Day()
		origMonth := timBase.Month()

		for {
			// Прибавляем один год
			timBase = timBase.AddDate(1, 0, 0)

			// Проверяем, совпадают ли месяц и день
			if timBase.Day() == origDay && timBase.Month() == origMonth {
				// Проверяем, что дата после текущей
				if timBase.After(now) {
					break
				}
			} else {
				// Если дата изменилась из-за високосного года, устанавливаем на 1 марта
				timBase = time.Date(timBase.Year(), time.March, 1, 0, 0, 0, 0, timBase.Location())
				if timBase.After(now) {
					break
				}
			}
		}
		return timBase.Format("20060102"), nil
	}

	if rep[0] == "d" {
		if len(rep) < 2 {
			return "", errors.New("некорректно указан режим повторения")
		}

		days, err := strconv.Atoi(rep[1])
		if err != nil {
			return "", err // Возвращаем ошибку, если количество дней некорректно
		}

		if days > 400 {
			return "", errors.New("перенос события более чем на 400 дней недопустим")
		}

		// Добавляем дни до тех пор, пока дата не станет после текущей
		for {
			timBase = timBase.AddDate(0, 0, days)
			if timBase.After(now) {
				break
			}
		}
		return timBase.Format("20060102"), nil
	}

	return "", errors.New("некорректное правило повторения")
}

// sendJSONError отправляет ошибку в формате JSON
func sendJSONError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(code)
	err := json.NewEncoder(w).Encode(map[string]string{"error": message})
	if err != nil {
		return
	}
}

// addTask добавляет новую задачу в базу данных
func addTask(w http.ResponseWriter, r *http.Request) {
	var task Task

	// Декодирование JSON-запроса
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		sendJSONError(w, "Ошибка десериализации JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Проверка обязательного поля Title
	if task.Title == "" {
		sendJSONError(w, "Не указан заголовок задачи", http.StatusBadRequest)
		return
	}

	// Установка текущей даты, если поле date не указано
	now := time.Now()
	nowDateStr := now.Format("20060102")
	now, _ = time.Parse("20060102", nowDateStr)

	if task.Date == "" {
		task.Date = nowDateStr
	}

	parsedDate, err := time.Parse("20060102", task.Date)
	if err != nil {
		sendJSONError(w, "Дата представлена в неправильном формате, ожидается YYYYMMDD", http.StatusBadRequest)
		return
	}

	if task.Repeat != "" {
		// Если задача повторяющаяся и дата в прошлом, вычисляем следующую дату
		if parsedDate.Before(now) {
			nextDate, err := NextDate(now, task.Date, task.Repeat)
			if err != nil {
				sendJSONError(w, "Правило повторения указано в неправильном формате: "+err.Error(), http.StatusBadRequest)
				return
			}
			task.Date = nextDate
		}
	} else {
		// Если задача не повторяющаяся и дата в прошлом, устанавливаем сегодняшнюю дату
		if parsedDate.Before(now) {
			task.Date = nowDateStr
		}
	}

	DbRep := NewRepo(db)

	idAdd, err := DbRep.addTaskToDB(task)
	task.ID = strconv.FormatInt(idAdd, 10)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	json.NewEncoder(w).Encode(map[string]string{"id": task.ID})
}

// getTasks извлекает задачи из базы данных и возвращает их в формате JSON.
func getTasks(w http.ResponseWriter, r *http.Request) {
	DbRep := NewRepo(db)
	rows, err := DbRep.getTaskFromDB()
	if err != nil {
		http.Error(w, "Ошибка при получении задач: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var tasks []map[string]string
	for rows.Next() {
		var id, date, title, comment, repeat string
		if err := rows.Scan(&id, &date, &title, &comment, &repeat); err != nil {
			http.Error(w, "Ошибка при сканировании задачи: "+err.Error(), http.StatusInternalServerError)
			return
		}
		tasks = append(tasks, map[string]string{
			"id":      id,
			"date":    date,
			"title":   title,
			"comment": comment,
			"repeat":  repeat,
		})
	}

	if err := rows.Err(); err != nil {
		http.Error(w, "Ошибка при чтении данных задач: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	err = json.NewEncoder(w).Encode(map[string]interface{}{"tasks": tasks})
	if err != nil {
		return
	}
}

// getTaskByID извлекает задачу по идентификатору из базы данных и возвращает ее в формате JSON.
func getTaskByID(w http.ResponseWriter, r *http.Request, id string) {

	var task Task

	DbRep := NewRepo(db)
	err := DbRep.checkTaskDone(id, &task)
	if err != nil {
		if err == sql.ErrNoRows {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			json.NewEncoder(w).Encode(map[string]string{"error": "Задача не найдена"})
		} else {
			http.Error(w, "Ошибка при получении задачи: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	err = json.NewEncoder(w).Encode(task)
	if err != nil {
		return
	}
}

// markTaskDone помечает задачу как выполненную.
func markTaskDone(w http.ResponseWriter, r *http.Request) {
	DbRep := NewRepo(db)
	id := r.URL.Query().Get("id")
	if id == "" {
		sendJSONError(w, "Не указан идентификатор задачи", http.StatusBadRequest)
		return
	}

	idInt, err := strconv.Atoi(id)
	if err != nil {
		sendJSONError(w, "Не корректно указан идентификатор", http.StatusBadRequest)
		return
	}

	var task Task
	err = DbRep.checkTaskDone(id, &task)
	if err != nil {
		if err == sql.ErrNoRows {
			sendJSONError(w, "Задача не найдена", http.StatusNotFound)
		} else {
			sendJSONError(w, "Ошибка при получении задачи: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	if task.Repeat == "" {

		// Одноразовая задача, удаляем ее из базы
		_, err = DbRep.Delete(idInt)
		if err != nil {
			sendJSONError(w, "Ошибка при удалении задачи: "+err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		// Периодическая задача, рассчитываем следующую дату
		// Используем дату задачи для вычисления следующей даты
		parsedDate, err := time.Parse("20060102", task.Date)
		if err != nil {
			sendJSONError(w, "Некорректный формат даты задачи", http.StatusInternalServerError)
			return
		}

		nextDate, err := NextDate(parsedDate, task.Date, task.Repeat)
		if err != nil {
			sendJSONError(w, "Ошибка при вычислении следующей даты: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Обновляем дату задачи на следующую дату
		_ = DbRep.UpdateNewDate(id, nextDate)
	}

	// Возвращаем пустой JSON при успешном завершении
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	err = json.NewEncoder(w).Encode(map[string]string{})
	if err != nil {
		return
	}
}

// tasksHandler обрабатывает GET-запросы к /api/tasks
func tasksHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendJSONError(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	// Устанавливаем значение лимита по умолчанию
	limit := 20 // Рекомендуемое количество задач

	// Получаем параметр limit из запроса, если он указан
	if limitParam := r.URL.Query().Get("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 {
			limit = l
		}
	}

	// Получаем параметры search и date из запроса
	search := r.URL.Query().Get("search")
	dateParam := r.URL.Query().Get("date")

	var tasks []Task
	var err error
	DbRep := NewRepo(db)
	tasks, err = DbRep.searchTaskFromDB(search, dateParam, limit)

	// Возвращаем задачи в формате JSON
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	err = json.NewEncoder(w).Encode(map[string]interface{}{"tasks": tasks})
	if err != nil {
		return
	}
}

// updateTask обновляет существующую задачу в базе данных
func updateTask(w http.ResponseWriter, r *http.Request) {
	var task Task

	// Декодирование JSON-запроса
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		sendJSONError(w, "Ошибка десериализации JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Проверка обязательного поля ID
	if task.ID == "" {
		sendJSONError(w, "Не указан идентификатор задачи", http.StatusBadRequest)
		return
	}

	// Проверка обязательного поля Title
	if task.Title == "" {
		sendJSONError(w, "Не указан заголовок задачи", http.StatusBadRequest)
		return
	}

	// Установка текущей даты, если поле date не указано
	now := time.Now()
	nowDateStr := now.Format("20060102")
	now, _ = time.Parse("20060102", nowDateStr)

	if task.Date == "" {
		task.Date = nowDateStr
	}

	parsedDate, err := time.Parse("20060102", task.Date)
	if err != nil {
		sendJSONError(w, "Дата представлена в неправильном формате, ожидается YYYYMMDD", http.StatusBadRequest)
		return
	}

	if task.Repeat != "" {
		// Если задача повторяющаяся и дата в прошлом, вычисляем следующую дату
		if parsedDate.Before(now) {
			nextDate, err := NextDate(now, task.Date, task.Repeat)
			if err != nil {
				sendJSONError(w, "Ошибка в правиле повторения: "+err.Error(), http.StatusBadRequest)
				return
			}
			task.Date = nextDate
		}
	} else {
		// Если задача не повторяющаяся и дата в прошлом, устанавливаем сегодняшнюю дату
		if parsedDate.Before(now) {
			task.Date = nowDateStr
		}
	}

	// Проверяем, существует ли задача с таким ID
	DbRep := NewRepo(db)
	err = DbRep.checkExistingID(task.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			sendJSONError(w, "Задача не найдена", http.StatusNotFound)
		} else {
			sendJSONError(w, "Ошибка при проверке задачи: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Обновление задачи в базе данных
	_ = DbRep.Update(task)

	// Возвращаем пустой JSON при успешном обновлении
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	err = json.NewEncoder(w).Encode(map[string]string{})
	if err != nil {
		return
	}
}

// taskHandler обрабатывает маршруты для /api/task
func taskHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		addTask(w, r)
	case http.MethodGet:
		if id := r.URL.Query().Get("id"); id != "" {
			getTaskByID(w, r, id)
		} else {
			// Возвращаем ошибку при отсутствии идентификатора
			sendJSONError(w, "Не указан идентификатор", http.StatusBadRequest)
		}
	case http.MethodPut:
		updateTask(w, r)
	case http.MethodDelete:
		// Обработчик для удаления задачи
		id := r.URL.Query().Get("id")
		if id == "" {
			sendJSONError(w, "Не указан идентификатор", http.StatusBadRequest)
			return
		}

		//преобразуем id в integer
		idInt, err := strconv.Atoi(id)
		if err != nil {
			sendJSONError(w, "Не корректно указан идентификатор", http.StatusBadRequest)
			return
		}

		DbRep := NewRepo(db)
		_, err = DbRep.Delete(idInt)
		if err != nil {
			sendJSONError(w, "Ошибка при удалении задачи: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Возвращаем пустой JSON при успешном удалении
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		err = json.NewEncoder(w).Encode(map[string]string{})
		if err != nil {
			return
		}

	default:
		sendJSONError(w, "Метод не разрешен", http.StatusMethodNotAllowed)
	}
}

func main() {
	if _, err := os.Stat("./scheduler.db"); os.IsNotExist(err) {
		createDB()
	}

	// Открытие базы данных
	var err error
	db, err = sql.Open("sqlite3", "./scheduler.db")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer db.Close()

	http.HandleFunc("/api/task", taskHandler)
	http.HandleFunc("/api/tasks", tasksHandler)
	http.HandleFunc("/api/task/done", markTaskDone)
	http.Handle("/", http.FileServer(http.Dir("./web")))
	http.HandleFunc("/api/nextdate", nextDateHandler)
	log.Fatal(http.ListenAndServe(":7540", nil))
}
