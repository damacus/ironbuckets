sed -i -e '/<<<<<<< HEAD/d' -e '/=======/d' -e '/>>>>>>> 91b9a41.*/d' internal/handlers/groups_handler_test.go
sed -i -e '/<<<<<<< HEAD/d' -e '/=======/d' -e '/>>>>>>> 91b9a41.*/d' internal/handlers/users_handler_test.go

gofmt -s -w internal/handlers/groups_handler_test.go internal/handlers/users_handler_test.go
