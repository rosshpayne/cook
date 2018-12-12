aws dynamodb create-table \
    --table-name Activity  \
    --attribute-definitions \
        AttributeName=rId,AttributeType=S \
        AttributeName=aId,AttributeType=N \
    --key-schema \
        AttributeName=rId,KeyType=HASH \
        AttributeName=aId,KeyType=RANGE \
    --provisioned-throughput \
        ReadCapacityUnits=1,WriteCapacityUnits=1