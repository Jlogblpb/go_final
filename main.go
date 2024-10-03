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

// createDB инициализирует базу данных SQLite и создает таблицу.
func createDB() {
	db, err := sql.Open("sqlite3", "./scheduler.db")
	if err != nil {
		log.Fatalf("Ошибка при открытии базы данных: %v", err)
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
			log.Fatalf("Ошибка при выполнении команды: %s, ошибка: %v", cmd, err)
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

// Функция для вычисления следующей даты на основе правила повторения
func NextDate(now time.Time, date string, repeat string) (string, error) {
	fmt.Println()
	fmt.Println()
	fmt.Println("======= Принятые значения: Сейчас:", now.Format("20060102"), "--Старт:", date, "--", repeat, "===========")

	// Проверяем наличие правила повторения, если его нет - возвращаем ошибку
	if repeat == "" {
		fmt.Println("======= ОШИБКА: Правило повторения отсутствует! ===========")
		return "", errors.New("правило повторения отсутствует")
	}

	rep := strings.Split(repeat, " ")
	fmt.Println("======= Парсим правило повторения:", rep, "===========")

	if len(rep) < 1 || (rep[0] != "y" && rep[0] != "d") {
		fmt.Println("======= ОШИБКА: Неподдерживаемое правило повторения! ===========")
		return "правило повторения указано в неправильном формате", errors.New("правило повторения указано в неправильном формате")
	}

	// Парсим дату события в формате YYYYMMDD
	timBase, err := time.Parse("20060102", date)
	if err != nil {
		fmt.Println("======= ОШИБКА: Некорректная дата! ===========")
		return "", err
	}
	fmt.Println("======= Дата успешно распознана:", timBase, "===========")

	// Проверяем режим повторения: год или день
	if rep[0] == "y" {
		// Если год, то прибавляем к дате год, пока не найдем следующую дату после текущей
		fmt.Println("======= Определяем режим повтора: год (y) ===========")
		timBase = timBase.AddDate(1, 0, 0) // Добавляем один год
		for timBase.Before(now) {
			timBase = timBase.AddDate(1, 0, 0) // Добавляем один год
			fmt.Println("======= Добавляем 1 год! Новая дата:", timBase.Format("20060102"), "===========")
		}
		result := timBase.Format("20060102")
		fmt.Println("======= Старая дата:", date, "===========")
		fmt.Println("=======  Новая дата:", result, "===========")
		fmt.Println("=========== Добавить: 1 год============")
		fmt.Println("=======  Новая дата:", result, "===========")
		return result, nil
	}

	if rep[0] == "d" {
		// Если день, то прибавляем указанное количество дней
		fmt.Println("======= Определяем режим повтора: день (d) ===========")
		if len(rep) < 2 {
			fmt.Println("======= ОШИБКА: Некорректно указан режим повторения! ===========")
			return "", errors.New("некорректно указан режим повторения")
		}

		days, err := strconv.Atoi(rep[1])
		if err != nil {
			return "", err // Возвращаем ошибку, если количество дней некорректно
		}

		if days > 400 {
			fmt.Println("======= ОШИБКА: Количество дней превышает 400! ===========")
			return "", errors.New("перенос события более чем на 400 дней недопустим")
		}

		fmt.Println("======= Количество дней для добавления:", days, "===========")
		if days == 1 && now.Format("20060102") == timBase.Format("20060102") {
			fmt.Println("")
			fmt.Println("***********************************************")
			fmt.Println("======= ", now.Format("20060102"), " = ", timBase.Format("20060102"), "===========")
			fmt.Println("======= ВСЕГО 1 ДЕНЬ!!!! ВЫХОДИМ ===========")
			fmt.Println("***********************************************")
			fmt.Println("")
		} else {
			fmt.Println("======= ВСЕГО", days, "ДНЕЙ!!!! ВЫХОДИМ ===========")
			timBase = timBase.AddDate(0, 0, days)
			for timBase.Before(now) {
				timBase = timBase.AddDate(0, 0, days)
				fmt.Println("======= Добавляем", days, "дней! Новая дата:", timBase.Format("20060102"), "===========")
			}
		}

		result := timBase.Format("20060102")
		fmt.Println("=======   Текущая дата:", now.Format("20060102"), "===========")
		fmt.Println("======= Стартовая дата:", date, "===========")
		fmt.Println("============= Добавить:", days, " дней==========")
		fmt.Println("=======     Новая дата:", result, "===========")
		return result, nil
	}

	return "", errors.New("некорректное правило повторения")
}

// sendJSONError отправляет ошибку в формате JSON
func sendJSONError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// addTask добавляет новую задачу в базу данных
func addTask(w http.ResponseWriter, r *http.Request) {
	var task Task

	// Декодирование JSON-запроса
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		sendJSONError(w, "Ошибка десериализации JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Println()
	fmt.Println("------ Принятые значения:", "--Старт:", task.Date, "---", task.Repeat, "---", task.Title, "---", task.Comment, "===========")
	fmt.Println()

	// Проверка обязательного поля Title
	if task.Title == "" {
		sendJSONError(w, "Не указан заголовок задачи", http.StatusBadRequest)
		return
	}

	// Установка текущей даты, если поле date не указано
	now := time.Now()
	if task.Date == "" {
		task.Date = now.Format("20060102")
	} else {
		parsedDate, err := time.Parse("20060102", task.Date)
		if err != nil {
			sendJSONError(w, "Дата представлена в неправильном формате, ожидается YYYYMMDD", http.StatusBadRequest)
			return
		}

		if parsedDate.Before(now) {
			if task.Repeat == "" {
				task.Date = now.Format("20060102")
			} else {
				nextDate, err := NextDate(now, task.Date, task.Repeat)
				if err != nil {
					sendJSONError(w, "Ошибка в правиле повторения: "+err.Error(), http.StatusBadRequest)
					return
				}
				task.Date = nextDate
			}
		}
	}

	// Проверка правила повторения
	if task.Repeat != "" {
		if _, err := NextDate(now, task.Date, task.Repeat); err != nil {
			sendJSONError(w, "Правило повторения указано в неправильном формате: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Открытие базы данных
	db, err := sql.Open("sqlite3", "./scheduler.db")
	if err != nil {
		sendJSONError(w, "Ошибка сервера: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Вставка новой задачи в базу данных
	stmt, err := db.Prepare("INSERT INTO scheduler(date, title, comment, repeat) VALUES (?, ?, ?, ?)")
	if err != nil {
		sendJSONError(w, "Ошибка при подготовке запроса: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	res, err := stmt.Exec(task.Date, task.Title, task.Comment, task.Repeat)
	if err != nil {
		sendJSONError(w, "Ошибка при вставке задачи: "+err.Error(), http.StatusInternalServerError)
		return
	}

	id, err := res.LastInsertId()
	if err != nil {
		sendJSONError(w, "Ошибка при получении ID задачи: "+err.Error(), http.StatusInternalServerError)
		return
	}

	task.ID = strconv.FormatInt(id, 10)

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	json.NewEncoder(w).Encode(map[string]string{"id": task.ID})
}

// getTasks извлекает задачи из базы данных и возвращает их в формате JSON.
func getTasks(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "./scheduler.db")
	if err != nil {
		http.Error(w, "Ошибка сервера: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, date, title, comment, repeat FROM scheduler ORDER BY date ASC")
	if err != nil {
		http.Error(w, "Ошибка при получении задач: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

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
	json.NewEncoder(w).Encode(map[string]interface{}{"tasks": tasks})
}

// getTaskByID извлекает задачу по идентификатору из базы данных и возвращает ее в формате JSON.
func getTaskByID(w http.ResponseWriter, r *http.Request, id string) {
	db, err := sql.Open("sqlite3", "./scheduler.db")
	if err != nil {
		http.Error(w, "Ошибка сервера: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	var task struct {
		ID      string `json:"id"`
		Date    string `json:"date"`
		Title   string `json:"title"`
		Comment string `json:"comment"`
		Repeat  string `json:"repeat"`
	}

	err = db.QueryRow("SELECT id, date, title, comment, repeat FROM scheduler WHERE id = ?", id).Scan(&task.ID, &task.Date, &task.Title, &task.Comment, &task.Repeat)
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
	json.NewEncoder(w).Encode(task)
}

// markTaskDone обрабатывает POST-запрос для выполнения задачи.
func markTaskDone(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		json.NewEncoder(w).Encode(map[string]string{"error": "Не указан идентификатор"})
		return
	}

	db, err := sql.Open("sqlite3", "./scheduler.db")
	if err != nil {
		http.Error(w, "Ошибка сервера: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	var task struct {
		ID      string `json:"id"`
		Date    string `json:"date"`
		Title   string `json:"title"`
		Comment string `json:"comment"`
		Repeat  string `json:"repeat"`
	}

	err = db.QueryRow("SELECT id, date, title, comment, repeat FROM scheduler WHERE id = ?", id).Scan(&task.ID, &task.Date, &task.Title, &task.Comment, &task.Repeat)
	if err != nil {
		if err == sql.ErrNoRows {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			json.NewEncoder(w).Encode(map[string]string{"error": "Задача не найдена"})
		} else {
			http.Error(w, "Ошибка при получении задачи: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Если задача не имеет правила повторения, удаляем ее.
	if task.Repeat == "" {
		_, err = db.Exec("DELETE FROM scheduler WHERE id = ?", id)
		if err != nil {
			http.Error(w, "Ошибка при удалении задачи: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		json.NewEncoder(w).Encode(map[string]string{})
		return
	}

	// Если задача имеет правило повторения, просто возвращаем успешный ответ.
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	json.NewEncoder(w).Encode(map[string]string{})
}

// tasksHandler обрабатывает GET-запросы к /api/tasks
func tasksHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendJSONError(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	db, err := sql.Open("sqlite3", "./scheduler.db")
	if err != nil {
		sendJSONError(w, "Ошибка сервера: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer db.Close()

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

	var rows *sql.Rows

	// Построение SQL-запроса на основе параметров
	if search != "" {
		// Поиск по подстроке в полях title и comment
		searchPattern := "%" + search + "%"
		query := "SELECT id, date, title, comment, repeat FROM scheduler WHERE title LIKE ? OR comment LIKE ? ORDER BY date ASC LIMIT ?"
		rows, err = db.Query(query, searchPattern, searchPattern, limit)
	} else if dateParam != "" {
		// Фильтрация по дате
		date := dateParam
		if len(dateParam) == 10 && strings.Contains(dateParam, ".") {
			// Преобразование даты из формата DD.MM.YYYY в YYYYMMDD
			t, err := time.Parse("02.01.2006", dateParam)
			if err != nil {
				sendJSONError(w, "Некорректный формат даты, ожидается YYYYMMDD или DD.MM.YYYY", http.StatusBadRequest)
				return
			}
			date = t.Format("20060102")
		}

		query := "SELECT id, date, title, comment, repeat FROM scheduler WHERE date = ? ORDER BY date ASC LIMIT ?"
		rows, err = db.Query(query, date, limit)
	} else {
		// Получение всех задач, отсортированных по дате
		query := "SELECT id, date, title, comment, repeat FROM scheduler ORDER BY date ASC LIMIT ?"
		rows, err = db.Query(query, limit)
	}

	if err != nil {
		sendJSONError(w, "Ошибка при получении задач: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Инициализируем слайс задач как пустой, чтобы не получить null в JSON
	tasks := make([]Task, 0)

	for rows.Next() {
		var task Task
		var id int64
		if err := rows.Scan(&id, &task.Date, &task.Title, &task.Comment, &task.Repeat); err != nil {
			sendJSONError(w, "Ошибка при чтении задачи: "+err.Error(), http.StatusInternalServerError)
			return
		}
		task.ID = strconv.FormatInt(id, 10)
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		sendJSONError(w, "Ошибка при обработке задач: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Возвращаем задачи в формате JSON
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	json.NewEncoder(w).Encode(map[string]interface{}{"tasks": tasks})
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
	if task.Date == "" {
		task.Date = now.Format("20060102")
	} else {
		parsedDate, err := time.Parse("20060102", task.Date)
		if err != nil {
			sendJSONError(w, "Дата представлена в неправильном формате, ожидается YYYYMMDD", http.StatusBadRequest)
			return
		}

		if parsedDate.Before(now) {
			if task.Repeat == "" {
				task.Date = now.Format("20060102")
			} else {
				nextDate, err := NextDate(now, task.Date, task.Repeat)
				if err != nil {
					sendJSONError(w, "Ошибка в правиле повторения: "+err.Error(), http.StatusBadRequest)
					return
				}
				task.Date = nextDate
			}
		}
	}

	// Проверка правила повторения
	if task.Repeat != "" {
		if _, err := NextDate(now, task.Date, task.Repeat); err != nil {
			sendJSONError(w, "Правило повторения указано в неправильном формате: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Открытие базы данных
	db, err := sql.Open("sqlite3", "./scheduler.db")
	if err != nil {
		sendJSONError(w, "Ошибка сервера: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Проверяем, существует ли задача с таким ID
	var existingID string
	err = db.QueryRow("SELECT id FROM scheduler WHERE id = ?", task.ID).Scan(&existingID)
	if err != nil {
		if err == sql.ErrNoRows {
			sendJSONError(w, "Задача не найдена", http.StatusNotFound)
		} else {
			sendJSONError(w, "Ошибка при проверке задачи: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Обновление задачи в базе данных
	stmt, err := db.Prepare("UPDATE scheduler SET date = ?, title = ?, comment = ?, repeat = ? WHERE id = ?")
	if err != nil {
		sendJSONError(w, "Ошибка при подготовке запроса: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(task.Date, task.Title, task.Comment, task.Repeat, task.ID)
	if err != nil {
		sendJSONError(w, "Ошибка при обновлении задачи: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Возвращаем пустой JSON при успешном обновлении
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	json.NewEncoder(w).Encode(map[string]string{})
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

	default:
		sendJSONError(w, "Метод не разрешен", http.StatusMethodNotAllowed)
	}
}

func main() {
	if _, err := os.Stat("./scheduler.db"); os.IsNotExist(err) {
		createDB()
	}

	http.HandleFunc("/api/task", taskHandler)
	http.HandleFunc("/api/tasks", tasksHandler)
	http.HandleFunc("/api/task/done", markTaskDone)
	http.Handle("/", http.FileServer(http.Dir("./web")))
	http.HandleFunc("/api/nextdate", nextDateHandler)
	log.Fatal(http.ListenAndServe(":7540", nil))
}
