package postgres

import (
	"PostAndComment/graph/model"
	"PostAndComment/storage"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type PostgresStorage struct {
	db *sql.DB
}

func New(db *sql.DB) storage.Storage {
	return &PostgresStorage{db: db}
}

func (s *PostgresStorage) GetCommentsTree(postID string, limit, offset int32) ([]*model.Comment, error) {
	// Проверка существования поста
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM posts WHERE id = $1)", postID).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("failed to check post existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("post with ID %s not found", postID)
	}

	rows, err := s.db.Query(`
        SELECT id, post_id, parent_id, text, created_at
        FROM comments
        WHERE post_id = $1
        ORDER BY created_at
    `, postID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Группируем комментарии по parent_id
	allComments := make(map[string]*model.Comment)
	repliesnMap := make(map[string][]*model.Comment)
	var rootComments []*model.Comment

	for rows.Next() {
		var c model.Comment
		var parent sql.NullString
		var createdAt time.Time

		if err := rows.Scan(&c.ID, &c.PostID, &parent, &c.Text, &createdAt); err != nil {
			return nil, err
		}
		c.CreatedAt = createdAt.Format(time.RFC3339)
		if parent.Valid {
			c.ParentID = &parent.String
		}

		allComments[c.ID] = &c

		if parent.Valid {
			repliesnMap[parent.String] = append(repliesnMap[parent.String], &c)
		} else {
			rootComments = append(rootComments, &c)
		}
	}

	// Выбиаем limit комментариев начиная с offset
	if int(offset) >= len(rootComments) {
		return []*model.Comment{}, nil
	}

	end := int(offset + limit)
	if end > len(rootComments) {
		end = len(rootComments)
	}

	paginatedRoots := rootComments[offset:end]

	// Рекурсивно находим ответы к комментриям
	var attachChildren func(comment *model.Comment)
	attachChildren = func(comment *model.Comment) {
		if children, exists := repliesnMap[comment.ID]; exists {
			comment.Replies = children
			for _, child := range children {
				attachChildren(child)
			}
		}
	}

	for _, root := range paginatedRoots {
		attachChildren(root)
	}

	return paginatedRoots, nil
}

// Создание поста
func (s *PostgresStorage) NewPost(text string, commentsEnabled bool) (*model.Post, error) {
	id := uuid.New().String()

	createdTime := time.Now().Format(time.RFC3339)

	_, err := s.db.Exec(`
		INSERT INTO posts (id, text, comments_enabled, created_at)
		VALUES ($1, $2, $3, $4)`,
		id, text, commentsEnabled, createdTime)
	if err != nil {
		return nil, err
	}

	return &model.Post{
		ID:              id,
		Text:            text,
		CommentsEnabled: commentsEnabled,
		CreatedAt:       createdTime,
	}, nil
}

// Доабвление комментария
func (s *PostgresStorage) AddComment(postID string, parentID *string, text string) (*model.Comment, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Транзакция проверки существования поста и комментрия
	var exists bool
	err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM posts WHERE id = $1 AND comments_enabled)", postID).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("failed to check post existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("post with ID %s not found or comments are disabled", postID)
	}

	if parentID != nil {
		err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM comments WHERE id = $1)", *parentID).Scan(&exists)
		if err != nil {
			return nil, fmt.Errorf("failed to check parent comment: %w", err)
		}
		if !exists {
			return nil, fmt.Errorf("parent comment with ID %s not found", *parentID)
		}
	}

	id := uuid.New().String()
	createdAt := time.Now().Format(time.RFC3339)
	//Добовляем комментарий
	_, err = tx.Exec(`
        INSERT INTO comments (id, post_id, parent_id, text, created_at)
        VALUES ($1, $2, $3, $4, $5)
    `, id, postID, parentID, text, createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert comment: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	newComment := &model.Comment{
		ID:        id,
		PostID:    postID,
		ParentID:  parentID,
		Text:      text,
		CreatedAt: createdAt,
	}
	return newComment, nil
}

// Список из limit постов начиная с offset
func (s *PostgresStorage) GetPosts(limit, offset int32) ([]*model.Post, error) {

	rows, err := s.db.Query(`
		SELECT id, text, comments_enabled, created_at
		FROM posts
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var posts []*model.Post
	for rows.Next() {
		var post model.Post
		var createdAt time.Time
		if err := rows.Scan(&post.ID, &post.Text, &post.CommentsEnabled, &createdAt); err != nil {
			return nil, err
		}
		post.CreatedAt = createdAt.Format(time.RFC3339)
		posts = append(posts, &post)
	}
	return posts, nil
}

// Запрос поста по ID
func (s *PostgresStorage) GetPost(postID string) (*model.Post, error) {
	var post model.Post
	var createdAt time.Time
	err := s.db.QueryRow(`
		SELECT id, text, comments_enabled, created_at
		FROM posts
		WHERE id = $1
	`, postID).Scan(&post.ID, &post.Text, &post.CommentsEnabled, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("post with ID %s not found", postID)
		}
		return nil, err
	}
	post.CreatedAt = createdAt.Format(time.RFC3339)
	return &post, nil
}

func (s *PostgresStorage) SetCommentsEnabled(postID string, enabled bool) (*model.Post, error) {
	result, err := s.db.Exec("UPDATE posts SET comments_enabled = $1 WHERE id = $2", enabled, postID)
	if err != nil {
		return nil, fmt.Errorf("failed to update post: %w", err)
	}

	rowsAf, err := result.RowsAffected() //Проверяем изменения
	if err != nil {
		return nil, fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rowsAf == 0 {
		return nil, fmt.Errorf("post with ID %s not found", postID)
	}

	return s.GetPost(postID)
}

// Подписка на комментарии к посту
func (s *PostgresStorage) SubscribeToComments(postID string) (<-chan *model.Comment, *func(), error) {
	// Проверяем существование поста
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM posts WHERE id = $1)", postID).Scan(&exists)
	if err != nil {
		return nil, nil, err
	}
	if !exists {
		return nil, nil, fmt.Errorf("post with ID %s not found", postID)
	}

	ch := make(chan *model.Comment, 5) // Канал с оповещениями
	done := make(chan struct{})        // Канал для закрытия горутины

	lastCheck := time.Now()
	// Проверяем каждые 4 секунды новые комментарии к посту
	go func() {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		defer close(ch)

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				rows, err := s.db.Query(`
                    SELECT id, post_id, parent_id, text, created_at
                    FROM comments
                    WHERE post_id = $1 AND created_at > $2
                    ORDER BY created_at
                `, postID, lastCheck.Format(time.RFC3339))

				if err != nil {
					continue
				}

				func() { // Обернул в функцию, чтобы гарантированно закрылся rows
					defer rows.Close()
					for rows.Next() {
						var c model.Comment
						var parent sql.NullString
						var createdAt time.Time

						if err := rows.Scan(&c.ID, // Копируем найденный комментарий
							&c.PostID,
							&parent,
							&c.Text,
							&createdAt); err != nil {
							continue
						}

						if parent.Valid { // Обработка ответов
							c.ParentID = &parent.String
						} else {
							c.ParentID = nil
						}
						c.CreatedAt = createdAt.Format(time.RFC3339)

						select {
						case ch <- &c:
							lastCheck = createdAt // Обновляем время проверки
						case <-done:
							return
						}
					}
				}()
			}
		}
	}()

	unsubscribe := func() { // Возвращаем функцию отписки
		close(done)
	}

	return ch, &unsubscribe, nil
}
