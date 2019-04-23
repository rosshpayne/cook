

module.exports = (header, subhdr, dataA, dataB, dataC, verbal, text, hint, height ) => { return {
    type: 'Alexa.Presentation.APL.RenderDocument',
    token: 'cook-tripple-screen',
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
              headerBackgroundColor: "blue",
              headerBackButton: true,
              headerNavigationAction: "backButton"
              },
              {
              type: "Sequence",
              scrollDirection: "vertical",
              numbered: true,
              grow: 1,
              shrink: 1,
              width: "100vw",
              height: "100vh",
              items: [
                        {
                        type: "Container",
                        direction: "column",
                        data: "${payload.listdata.properties.dataA}",
                        spacing: 0,
                        height: "12vh",
                        alignItems: "left",
                        justifyContent: "end",
                        items: [
                                          {
                                          type: "Text",
                                          shrink: "1",
                                          grow: "1",
                                          text: "${data.Title}",
                                          fontSize: "20dp"
                                          }
                                          ] 
                        },
                        {
                        type: "Frame",
                        borderWidth: 2,
                        height: height,  
                        item: {
                              type: "Container",
                              direction: "column",
                              spacing: 20,
                              alignItems: "left",
                              justifyContent: "center",
                              items: [
                                      {
                                      type: "Text",
                                      id: "Rinstruction",
                                      text: "  ${payload.listdata.properties.text}",
                                      speech: "${payload.listdata.properties.verbal}",
                                      fontSize: "37dp",
                                      style: "textStylePrimary1"
                                      }
                                     ] 
                        }
                        },
                        {
                        type: "Container",
                        direction: "column",
                        data: "${payload.listdata.properties.dataC}",
                        spacing: 20,
                        height: "40vh",
                        alignItems: "left",
                        items: [
                                          {
                                          type: "Text",
                                          text: "${data.Title}",
                                          grow: 1,
                                          shrink: 1,
                                          fontSize: "20dp"
                                          }
                                          ] 
                        }
                        ]
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
              dataA,
              dataB,
              dataC,
              verbal,
              text
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