REGION:=us-west-2

demo:
	apex deploy -r $(REGION) --env demo

demologs:
	apex logs tel -r $(REGION) --env demo -f

prod:
	apex deploy -r ap-southeast-1 --env prod

prodlogs:
	apex logs tel -r ap-southeast-1 --env prod

dev:
	apex deploy -r $(REGION) --env dev

devlogs:
	apex logs tel -r $(REGION) --env dev

test:
	apex --env dev -r $(REGION) invoke tel < functions/tel/sns.json

testprod:
	apex --env prod -r ap-southeast-1 invoke tel < functions/tel/sns.json
