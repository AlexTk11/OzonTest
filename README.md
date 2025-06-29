Запуск:
    
    1) С in-memory хранилищем: docker-compose --profile memory up --build
    
    2) C Postgres хранилищем: docker-compose --profile postgres up --build


Запуск тестов:
    1) storage/postgres:
    
    Перед запуском postgres_test.go запустить контейнер с БД:
     
     docker-compose -f docker-compose.test.yml up -d
     
     go test ./tests -run TestPostgresStorageTestSuite -v
    
    2)  storage/memory:
     
     go test ./tests -run TestMemoryStorageTestSuite -v
