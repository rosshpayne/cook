

module.exports = (data) => { return {
    type: 'Alexa.Presentation.APL.RenderDocument',
    token: 'splash-screen',
    document: {
      type: 'APL',
      version: '1.0',
      import: [
        {
          name: 'alexa-styles',
          version: '1.0.0'
        },
        {
          name: 'alexa-layouts',
          version: '1.0.0'
        }
      ],
      resources:  [],
      mainTemplate: {
        parameters: ['payload'],
        items: [
                {
                type: "Sequence",
                data: "${payload.listdata.properties.data.item}",
                scrollDirection: "vertical",
                width: "100vw",
                height: "100vh",
                items: [{
                  type: "Text",
                  text: "asdf metoo. ${data}"
                }
                ]
              }
              ]
      }
    },
    datasources: {
      listdata :    {        
            type: "object",
            properties: { 
              data
            }
      }
    }
  };
};