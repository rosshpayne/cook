aws dynamodb create-table \
    --table-name Unit \
    --attribute-definitions \
        AttributeName=unit,AttributeType=S \
    --key-schema \
        AttributeName=unit,KeyType=HASH \
    --provisioned-throughput \
        ReadCapacityUnits=1,WriteCapacityUnits=1
