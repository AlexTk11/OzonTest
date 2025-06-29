package memory

import (
	"PostAndComment/graph/model"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type InMemoryStorage struct {
	mu                  sync.RWMutex
	posts               []*model.Post          //Список постов
	postsCommentsEnable map[string]bool        //Признак включенных комментариев + Проверка существования поста
	postSearch          map[string]*model.Post //Быстрый поиск постов по ID

	commentsAvaliability map[string]bool //Наличие комментария (Для проверки существования)

	commentsByPostAndParent map[string]map[string][]*model.Comment //Быстрый поиск комментария
	subscribers             map[string][]chan *model.Comment       //Подписчики на комментарии к посту
}

func New() *InMemoryStorage {
	return &InMemoryStorage{
		posts:                   make([]*model.Post, 0),
		postSearch:              make(map[string]*model.Post),
		postsCommentsEnable:     make(map[string]bool),
		commentsAvaliability:    make(map[string]bool),
		commentsByPostAndParent: make(map[string]map[string][]*model.Comment),
		subscribers:             make(map[string][]chan *model.Comment),
	}
}

const rootKey = "root" //Ключ родительского комментария для комментариев непосредственно к посту

// Создание поста
func (s *InMemoryStorage) NewPost(text string, commentsEnabled bool) (*model.Post, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	post := &model.Post{
		ID:              uuid.New().String(),
		Text:            text,
		CommentsEnabled: commentsEnabled,
		CreatedAt:       time.Now().Format(time.RFC3339),
	}

	s.posts = append(s.posts, post)
	s.postsCommentsEnable[post.ID] = commentsEnabled
	s.postSearch[post.ID] = post
	return post, nil
}

// Запрос поста по ID
func (s *InMemoryStorage) GetPost(postID string) (*model.Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	post, ok := s.postSearch[postID]
	if !ok {
		return nil, fmt.Errorf("post with ID %s not found", postID)
	}

	return post, nil
}

// Добавление комментария
func (s *InMemoryStorage) AddComment(postID string, parentID *string, text string) (*model.Comment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Проверка существования поста и доступности его комментирования
	comentsEnable, ok := s.postsCommentsEnable[postID]
	if !ok {
		return nil, fmt.Errorf("post with ID %s not found", postID)
	}
	if !comentsEnable {
		return nil, fmt.Errorf("comments are disabled for this post")
	}

	// Проверка существования родительского комментария
	parentKey := rootKey
	if parentID != nil {
		if _, ok := s.commentsAvaliability[*parentID]; !ok {
			return nil, fmt.Errorf("parent comment with ID %s not found", *parentID)
		}
		parentKey = *parentID
	}

	comment := &model.Comment{
		ID:        uuid.New().String(),
		PostID:    postID,
		ParentID:  parentID,
		Text:      text,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	//Добавляем комментарий в мапу
	if _, ok := s.commentsByPostAndParent[postID]; !ok {
		s.commentsByPostAndParent[postID] = make(map[string][]*model.Comment)
	}
	s.commentsByPostAndParent[postID][parentKey] = append(
		s.commentsByPostAndParent[postID][parentKey], comment)

	s.commentsAvaliability[comment.ID] = true //Обновили признак существования комментария

	if subscribers, ok := s.subscribers[postID]; ok { //Рассылка комментария подписчикам
		for _, ch := range subscribers {
			select {
			case ch <- comment:
				{
					fmt.Printf("New comment for %s\n", postID)
				}
			default:
			}
		}
	}

	return comment, nil
}

// Список из limit постов начиная с offset
func (s *InMemoryStorage) GetPosts(limit, offset int32) ([]*model.Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	totalPosts := len(s.posts)
	if int(offset) >= totalPosts {
		return []*model.Post{}, nil
	}

	maxPosts := totalPosts - int(offset)
	if int(limit) > maxPosts {
		limit = int32(maxPosts)
	}

	result := make([]*model.Post, 0, limit)

	// Выдаем новые посты первыми (проходим список с конца)
	for i := int(offset); i < int(offset)+int(limit); i++ {
		postIndex := totalPosts - 1 - i
		result = append(result, s.posts[postIndex])
	}

	return result, nil
}

// Подписка на уведомления про новые комментарии к посту
func (s *InMemoryStorage) SubscribeToComments(postID string) (<-chan *model.Comment, *func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.postsCommentsEnable[postID]; !ok {
		return nil, nil, fmt.Errorf("post with ID %s not found", postID)
	}

	ch := make(chan *model.Comment, 1)
	s.subscribers[postID] = append(s.subscribers[postID], ch)

	rmSubscription := func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		subs := s.subscribers[postID]
		for i, subscriber := range subs {
			if subscriber == ch {
				s.subscribers[postID] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
		close(ch)
	}

	return ch, &rmSubscription, nil
}

// Включение/выключение комментариев к посту
func (s *InMemoryStorage) SetCommentsEnabled(postID string, enabled bool) (*model.Post, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, post := range s.posts {
		if post.ID == postID {
			post.CommentsEnabled = enabled
			s.postsCommentsEnable[postID] = enabled
			return post, nil
		}
	}

	return nil, fmt.Errorf("post with ID %s not found", postID)
}

// Запрос комментариев к посту и ответов к ним
func (s *InMemoryStorage) GetCommentsTree(postID string, limit, offset int32) ([]*model.Comment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.postsCommentsEnable[postID]; !ok {
		return nil, fmt.Errorf("post with ID %s not found", postID)
	}

	postComments := s.commentsByPostAndParent[postID]
	if postComments == nil {
		return []*model.Comment{}, nil
	}

	// Получаем корневые комментарии
	rootComments := postComments[rootKey]

	if int(offset) >= len(rootComments) {
		return []*model.Comment{}, nil
	}

	end := int(offset + limit)
	if end > len(rootComments) {
		end = len(rootComments)
	}

	currentRootComments := rootComments[offset:end]

	// Рекурсивный обход ответов на корневые комментарии
	var attachChildren func(comment *model.Comment)
	attachChildren = func(comment *model.Comment) {
		if children, ok := postComments[comment.ID]; ok {
			comment.Replies = children
			for _, child := range children {
				attachChildren(child)
			}
		}
	}

	result := make([]*model.Comment, len(currentRootComments))
	for i, root := range currentRootComments {
		result[i] = &model.Comment{
			ID:        root.ID,
			PostID:    root.PostID,
			ParentID:  root.ParentID,
			Text:      root.Text,
			CreatedAt: root.CreatedAt,
		}
		attachChildren(result[i])
	}

	return result, nil
}
