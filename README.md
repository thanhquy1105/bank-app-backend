# simplebank
Golang, Gin, Docker, K8s, gRPC, Redis, Postgres

## Simple bank service

The service that we’re going to build is a simple bank. It will provide APIs for the frontend to do the following things:

- Create and manage bank accounts, which are composed of the owner’s name, balance, and currency.
- Record all balance changes to each of the accounts. So every time some money is added to or subtracted from the account, an account entry record will be created.
- Perform a money transfer between 2 accounts. This should happen within a transaction, so that either both accounts’ balances are updated successfully or none of them are.

## Database
https://dbdiagram.io/d/64d78c4802bd1c4a5eabec0b

## Clone
It should use a Unix-style line endings instead of Windows. This problem occurs some errors when running sh file in docker.
You should run the following command before cloning the repository:

```git config --global core.autocrlf false```
Then clone the repository and proceed.

## To run this simple bank project
There are 3 ways written in Makefile to run the project

- RUN SIMPLE_BANK USING POSTGRES CONTAINER AND REDIS CONTAINER AND ```make server``` TO START SERVER
- RUN SIMPLE_BANK USING DOCKER NETWORK TO CONNECT 3 STAND-ALONE CONTAINERS (postgres container + redis container + app container + network)
- RUN SIMPLE_BANK USING DOCKER COMPOSE UP
