# simplebank
Golang, Gin, Docker, K8s, gRPC, Redis, Postgres

## Simple bank service

The service that we’re going to build is a simple bank. It will provide APIs for the frontend to do the following things:

- Create and manage bank accounts, which are composed of the owner’s name, balance, and currency.
- Record all balance changes to each of the accounts. So every time some money is added to or subtracted from the account, an account entry record will be created.
- Perform a money transfer between 2 accounts. This should happen within a transaction, so that either both accounts’ balances are updated successfully or none of them are.

## Database
https://dbdiagram.io/d/64d78c4802bd1c4a5eabec0b
