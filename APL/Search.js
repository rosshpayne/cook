

module.exports = (backbtn, header, subhdr, data, verbal,  hint, err) => { return {
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
              headerSubtitle: subhdr,
              headerBackgroundColor: "green",
              headerBackButton: backbtn,
              headerNavigationAction: "backButton"
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
                       "select",
                       "${ordinal}" ,
                       "${data.Title}" 
                     ]
                  },
                  item: {
                        type: "Container",
                        direction: "column",
                        style: "textStylePressable",
                        spacing: 0,
                        height: 120,
                        alignItems: "left",
                       items: [
                                          {
                                          type: "Text",
                                          text: "${ordinal}. ${data.Title}",
                                          grow: 0,
                                          shrink: 1,
                                          spacing: 4,
                                          fontSize: "30dp"
                                          },
                                          {
                                          type: "Container",
                                          direction: "row",
                                          spacing: 0,
                                          width: "100vw",
                                          alignItems: "left",
                                          items:  [ {
                                                          type: "Text",
                                                          text: "${data.SubTitle1}",
                                                          grow: 0,
                                                          shrink: 0,
                                                          width: "50vw",
                                                          spacing: 40,
                                                          fontSize: "18dp"
                                                          },
                                                          {
                                                          type: "Text",
                                                          text: "${data.SubTitle2}",
                                                          width: "50vw",
                                                          spacing: 40,
                                                          grow: 0,
                                                          shrink: 0,
                                                          fontSize: "18dp"
                                                          }
                                                ]
                                          },
                                          {
                                          type: "Text",
                                          text: "   ${data.Text}",
                                          grow: 0,
                                          spacing: 5,
                                          shrink: 1,
                                          fontSize: "22dp"
                                          }
                                          ]   
                  }
                }
              },
              {
              type: "Text",
              id: "Rinstruction",
              speech: "${payload.listdata.properties.verbal}",
              fontSize: "37dp",
              style: "textStylePrimary1"
              },
              {
                type: "AlexaFooter",
                hintText: hint
              }
          ]
      }
      }
    },
    datasources: {
      listdata :    {        
            type: "object",
            properties: { 
              data,
              verbal
            },
            transformers: [{
                inputPath: "verbal",
                outputPath: "verbalOut",
                transformer: "ssmlToSpeech"
                },
                {
                inputPath: "verbal",
                outputPath: "text",
                transformer: "ssmlToText"
                }
            ]
      }
    }
  };
};