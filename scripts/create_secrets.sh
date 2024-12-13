#!/bin/bash

# .env 파일에서 Secret 생성
kubectl create secret generic app-secrets --from-env-file=.env.kubernates