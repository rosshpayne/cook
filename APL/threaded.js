

module.exports = (header, subhdr, dataA, dataB, dataC, verbal, text, dataD, dataE, dataF) => { return {
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
              headerBackgroundColor: "red",
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
                        spacing: 4,
                        height: "14vh",
                        alignItems: "left",
                        justifyContent: "end",
                        items: [
                                          {
                                          type: "Text",
                                          text: "${data.Title}",
                                          grow: 0,
                                          shrink: 0,
                                          fontSize: "15dp"
                                          }
                                          ] 
                        },
                        {
                        type: "Frame",
                        borderColor: "red",
                        borderWidth: 2,
                        height: "7vh",  
                        item: {
                              type: "Container",
                              direction: "column",
                              data: "${payload.listdata.properties.dataB}",
                              spacing: 4,
                              alignItems: "left",
                              height: "7vh",
                              justifyContent: "center",
                              items: [
                                      {
                                      type: "Text",
                                      id: "Rinstruction",
                                      text: "${data.Title}",
                                      grow: 0,
                                      shrink: 0,
                                      fontSize: "17dp",
                                      style: "textStylePrimary2"
                                      }
                                     ] 
                        }
                        },
                        {
                        type: "Container",
                        direction: "column",
                        data: "${payload.listdata.properties.dataC}",
                        spacing: 4,
                        height: "19vh",
                        alignItems: "left",
                        items: [
                                          {
                                          type: "Text",
                                          text: "${data.Title}",
                                          grow: 0,
                                          shrink: 0,
                                          fontSize: "15dp"
                                          }
                                          ] 
                        },
                        {
                        type: "Container",
                        direction: "column",
                        data: "${payload.listdata.properties.dataD}",
                        spacing: 4,
                        height: "14vh",
                        alignItems: "left",
                        justifyContent: "end",
                        items: [
                                          {
                                          type: "Text",
                                          text: "${data.Title}",
                                          grow: 0,
                                          shrink: 0,
                                          fontSize: "15dp"
                                          }
                                          ] 
                        },
                        {
                        type: "Frame",
                        borderColor: "red",
                        borderWidth: 2,
                        height: "7vh",  
                        item: {
                              type: "Container",
                              direction: "column",
                              spacing: 4,
                              alignItems: "left",
                              height: "7vh",
                              justifyContent: "center",
                              items: [
                                      {
                                      type: "Text",
                                      id: "Rinstruction",
                                      text: "  ${payload.listdata.properties.text}",
                                      speech: "${payload.listdata.properties.verbal}",
                                      grow: 0,
                                      shrink: 0,
                                      fontSize: "17dp",
                                      style: "textStylePrimary2"
                                      }
                                     ] 
                        }
                        },
                        {
                        type: "Container",
                        direction: "column",
                        data: "${payload.listdata.properties.dataF}",
                        spacing: 4,
                        height: "39vh",
                        alignItems: "left",
                        items: [
                                          {
                                          type: "Text",
                                          text: "${data.Title}",
                                          grow: 0,
                                          shrink: 0,
                                          fontSize: "15dp"
                                          }
                                          ] 
                        }
                        ]
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
              dataA,
              dataB,
              dataC,
              dataD,
              dataE,
              dataF,
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