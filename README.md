# points-api

## Dependencies
Docker

## How to Run 

- Git clone the repo 
- docker-compose up database 
- docker-compose up webserver
 
 ### Postman Testing 

- In `internal/src/sql` there is a postman collection of the existing endpoints. Perform a post to add-points to the database
- spend-points will reduce the remainder column in the database but should not go below zero
- If points are not available to spend an error is returned 