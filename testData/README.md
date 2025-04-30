curl.exe -X "POST" -H "Content-Type: application/json" -d "@addUserAdmin.json" http://localhost:8080/user

curl.exe -X "POST" -H "Content-Type: application/json" -d "@addUser.json" http://localhost:8080/user

curl.exe -X "POST" -H "Content-Type: application/json" -H "Authorization: Basic YWRtaW46c2ltcGxl" -d "@recordActionAdmin.json" http://localhost:8080/pocketMoney/addAction


curl.exe http://localhost:8080/users

curl.exe http://localhost:8080/health