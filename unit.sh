aws dynamodb create-table \
    --table-name Unit \
    --attribute-definitions \
        AttributeName=slabel,AttributeType=S \
    --key-schema \
        AttributeName=slabel,KeyType=HASH \
    --provisioned-throughput \
        ReadCapacityUnits=1,WriteCapacityUnits=1
