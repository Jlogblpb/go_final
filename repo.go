package main

import (
	"database/sql"
	"strconv"
	"strings"
	"time"
)

// Repo представляет структуру базы данных
type Repo struct {
	db *sql.DB
}

// Инициализируем базу данных
func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// Удаляем задачу из базы данных
func (rep *Repo) Delete(id int) (res int64, err error) {
	result, err := rep.db.Exec("DELETE FROM scheduler WHERE id = ?", id)
	if err != nil {
		return 0, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return rowsAffected, nil
}

// Проверяем, существует ли задача с таким ID
func (rep *Repo) checkExistingID(existingID string) (err error) {
	err = rep.db.QueryRow("SELECT id FROM scheduler WHERE id = ?", existingID).Scan(&existingID)
	if err != nil {
		return err
	}
	return nil
}

// Обновление задачи в базе данных
func (rep *Repo) Update(task Task) (err error) {
	stmt, err := rep.db.Prepare("UPDATE scheduler SET date = ?, title = ?, comment = ?, repeat = ? WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(task.Date, task.Title, task.Comment, task.Repeat, task.ID)
	if err != nil {
		return err
	}
	return nil
}

// Обновляем дату задачи на следующую дату
func (rep *Repo) UpdateNewDate(id string, nextDate string) (err error) {
	_, err = rep.db.Exec("UPDATE scheduler SET date = ? WHERE id = ?", nextDate, id)
	if err != nil {
		return err
	}
	return nil
}

// Проверяем выполнена ли задача
func (rep *Repo) checkTaskDone(id string, task *Task) (err error) {
	err = rep.db.QueryRow("SELECT id, date, title, comment, repeat FROM scheduler WHERE id = ?",
		id).Scan(&task.ID, &task.Date, &task.Title, &task.Comment, &task.Repeat)
	if err != nil {
		return err
	}
	return
}

// Извлекаем задачи из базы данных
func (rep *Repo) getTaskFromDB() (rows *sql.Rows, err error) {
	rows, err = rep.db.Query("SELECT id, date, title, comment, repeat FROM scheduler ORDER BY date ASC")
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// Добавляем задачу в базу данных
func (rep *Repo) addTaskToDB(task Task) (taskID int64, err error) {
	stmt, err := rep.db.Prepare("INSERT INTO scheduler(date, title, comment, repeat) VALUES (?, ?, ?, ?)")
	if err != nil {
		return
	}
	res, err := stmt.Exec(task.Date, task.Title, task.Comment, task.Repeat)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// Поиск по подстроке в полях title и comment
func (rep *Repo) searchTaskFromDB(search string, dateParam string, limit int) (tasks []Task, err error) {
	var rows *sql.Rows
	// Построение SQL-запроса на основе параметров
	if search != "" {
		// Поиск по подстроке в полях title и comment
		searchPattern := "%" + search + "%"
		query := "SELECT id, date, title, comment, repeat FROM scheduler WHERE title LIKE ? OR comment LIKE ? ORDER BY date ASC LIMIT ?"
		rows, err = rep.db.Query(query, searchPattern, searchPattern, limit)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
	} else if dateParam != "" {
		// Фильтрация по дате
		date := dateParam
		if len(dateParam) == 10 && strings.Contains(dateParam, ".") {
			// Преобразование даты из формата DD.MM.YYYY в YYYYMMDD
			t, err := time.Parse("02.01.2006", dateParam)
			if err != nil {
				return nil, err
			}
			date = t.Format("20060102")
		}

		query := "SELECT id, date, title, comment, repeat FROM scheduler WHERE date = ? ORDER BY date ASC LIMIT ?"
		rows, err = rep.db.Query(query, date, limit)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
	} else {
		// Получение всех задач, отсортированных по дате
		query := "SELECT id, date, title, comment, repeat FROM scheduler ORDER BY date ASC LIMIT ?"
		rows, err = rep.db.Query(query, limit)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
	}

	// Инициализируем слайс задач как пустой, чтобы не получить null в JSON
	tasks = make([]Task, 0)

	// Чтение задач из базы данных
	for rows.Next() {
		var task Task
		var id int64
		if err := rows.Scan(&id, &task.Date, &task.Title, &task.Comment, &task.Repeat); err != nil {
			return nil, err
		}
		defer rows.Close()
		task.ID = strconv.FormatInt(id, 10)
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tasks, nil
}
