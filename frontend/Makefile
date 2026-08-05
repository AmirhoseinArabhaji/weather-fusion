.PHONY: dev build lint format type-check install clean help

dev:
	npm run dev

build:
	npm run build

start:
	npm run start

lint:
	npm run lint

lint-fix:
	npm run lint -- --fix

format:
	npx prettier --write "src/**/*.{ts,tsx,css,json}"

type-check:
	npx tsc --noEmit

install:
	npm install

clean:
	rm -rf .next out

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	  | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'
