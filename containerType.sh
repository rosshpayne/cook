aws dynamodb create-table \
    --table-name ContainerType \
    --attribute-definitions \
        AttributeName=cId,AttributeType=S \
    --key-schema \
        AttributeName=cId,KeyType=HASH \
    --provisioned-throughput \
        ReadCapacityUnits=1,WriteCapacityUnits=1
