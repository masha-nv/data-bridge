###

quit sqlite3
.quit

###

enter sqlite3
sqlite3 dev.db

###

seed data (not in sqlite terminal)

sqlite3 dev.db < seed.sql
sqlite3 test.db < seed.sql

###

start backend
go run main.go

###

check db content
sqlite3 dev.db ->
run any command like
check all tables: SELECT name FROM sqlite_master WHERE type='table';

###

remove entries from table
DELETE FROM users;
