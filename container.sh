aws dynamodb create-table \
    --table-name Container \
    --attribute-definitions \
        AttributeName=rId,AttributeType=S \
        AttributeName=cId,AttributeType=S \
    --key-schema \
        AttributeName=rId,KeyType=HASH \
        AttributeName=cId,KeyType=RANGE \
    --provisioned-throughput \
        ReadCapacityUnits=1,WriteCapacityUnits=1
