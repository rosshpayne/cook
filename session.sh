aws dynamodb create-table \
    --table-name Sessions  \
    --attribute-definitions \
        AttributeName=Uid,AttributeType=S \
    --key-schema \
        AttributeName=Uid,KeyType=HASH \
    --provisioned-throughput \
        ReadCapacityUnits=1,WriteCapacityUnits=1