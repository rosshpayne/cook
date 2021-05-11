

module.exports = (bkbtn, header, subhdr, data, hint, err) => { return {
    type: 'Alexa.Presentation.APL.RenderDocument',
    token: 'splash-screen',
    document: {
      type: 'APL',
      version: '1.0',
      theme: "dark",
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
              headerBackButton: bkbtn,
              headerNavigationAction: "backButton"
              },
              {
                type: "Container",
                direction: "column",
                spacing: 1,
                height: "20vh",
                items: [
                                          {
                                          type: "Text",
                                          text: err,
                                          textAlignVertical: "center",
                                          grow: 1,
                                          shrink: 1,
                                          fontSize: "18dp"
                                          }
                       ]
              },
              {
              type: "Sequence",
              scrollDirection: "vertical",
              data: "${payload.listdata.properties.data}",
              numbered: true,
              grow: 1,
              shrink: 1,
              width: "100vw",
              height: "80vh",
              item: {
                    type: "Text",
                    text: "  ${data.Title}",
                    grow: 0,
                    shrink: 1,
                    spacing: 4,
                    fontSize: "24dp"
                  }
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
              data
            }
      }
    }
  };
};