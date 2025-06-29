package storage

import "PostAndComment/graph/model"

type Storage interface {
	NewPost(text string, commentsEnabled bool) (*model.Post, error) // Создание поста

	AddComment(postID string, parentID *string, text string) (*model.Comment, error) // Добавление комментария

	GetCommentsTree(postID string, limit, offset int32) ([]*model.Comment, error) // Комментарии (с ответами) для указанного поста

	GetPosts(limit, offset int32) ([]*model.Post, error) // Список постов

	GetPost(postID string) (*model.Post, error) // Пост с комментариями

	SetCommentsEnabled(postID string, enabled bool) (*model.Post, error) //Вкл./выкл. комментарии

	SubscribeToComments(postID string) (<-chan *model.Comment, *func(), error) //Подписка на комментарии к посту
}
