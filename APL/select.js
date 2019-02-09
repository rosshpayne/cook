

module.exports = (header, data) => { return {
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
      styles: {
        textStylePressable: {
          values: [
            { backgroundColor: "blue",
              borderColor: "yellow",
              color: "yellow"
            }
        ]
      }
      },
      mainTemplate: {
        parameters: ['payload'],
        items: {
          when: "${@viewportProfile != @hubRoundSmall}",
          type: "Container",
          height: "100vh",
          width: "100vw",
          direction: "column",
          items: [
              {
              type: "AlexaHeader",
              headerTitle: header,
              headerBackgroundColor: "red",
              headerSubtitle: "Header Subtitle"
              },
              {
              type: "Sequence",
              scrollDirection: "vertical",
              data: "${payload.listdata.properties.data}",
              numbered: true,
              grow: 1,
              shrink: 1,
              width: "100vw",
              height: "100vh",
              item: {
                  type: "TouchWrapper",
                  onPress: {
                     type: "SendEvent",
                     arguments : [
                       'ItemSelected',
                       '${ordinal}' ,
                       '${data.Title}' 
                     ]
                  },
                  item: {
                        type: "Container",
                        direction: "column",
                        style: "textStylePressable",
                        spacing: 0,
                        height: 120,
                        alignItems: "left",
                        item: {
                                  type: "Container",
                                  direction: "column",
                                  style: "textStylePressable",
                                  spacing: 0,
                                  alignItems: "left",
                                  items: [
                                          {
                                          type: "Text",
                                          text: "${ordinal}. ${data.Title}",
                                          grow: 0,
                                          shrink: 1,
                                          spacing: 4,
                                          fontSize: "30dp"
                                          }
                                          ]   
                        }
                  }
                }
              },
              {
                type: "AlexaFooter",
                hintText: "hint text goes here.."
              }
          ]
      }
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