acp-f:
ifndef m
	git add . && git commit -m 'ok'  && git push
else
	git add . && git commit -m '$(m)'  && git push
endif