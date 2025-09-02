.PHONY: install-docker-plugin
install-docker-plugin:
	cp docker-pussh ~/.docker/cli-plugins/docker-pussh

.PHONY: shellcheck
shellcheck:
	find . -path "./tmp" -prune -o -type f \( -name "docker-pussh" -o -name "*.sh" \) -print0 \
		| xargs -0 shellcheck --enable=all ;

.PHONY: test
test:
	cd test && go test -v -count=1 ./...
