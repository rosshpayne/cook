aws dynamodb create-table \
    --table-name Sessions  \
    --attribute-definitions \
        AttributeName=PKey,AttributeType=S \
    --key-schema \
        AttributeName=PKey,KeyType=HASH \
    --provisioned-throughput \
        ReadCapacityUnits=1,WriteCapacityUnits=1