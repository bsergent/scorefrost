# ScoreFrost
Lightweight leaderboard and analytics solution for game jams.

## Developing Locally
Only start database and run API locally:
`docker-compose up -d db`
`go run main.go`

Stop database
`docker-compose down`

## Deploying
`docker build -t scoreforst:dev .`

`docker-compose up -d` Run the volume with a detached head.

`docker-compose up --build` Rebuild the GoAPI and run both the database and API.

`docker-compose down` Shut database and API down.

`docker ps` Check for running containers.

`docker stop <id>`

`docker system prune`

