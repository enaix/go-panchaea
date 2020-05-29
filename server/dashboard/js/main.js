Vue.use(Toasted, {
  action : {
    text: '●',
    onClick : (e, toastObject) => {
      toastObject.goAway(0)
    },
  },
  duration: 10000,
})

Vue.component('node', {
  props: ['client'],
  template: `
  <div class="node c0-bg-h c15-fg">
    <div class="row">
      <div class="col-6 node-wrapper">
        <svg class="bi bi-box" width="2.3rem" height="2.3rem" viewBox="0 0 16 16" fill="currentColor" xmlns="http://www.w3.org/2000/svg">
          <path fill-rule="evenodd" d="M8.186 1.113a.5.5 0 0 0-.372 0L1.846 3.5 8 5.961 14.154 3.5 8.186 1.113zM15 4.239l-6.5 2.6v7.922l6.5-2.6V4.24zM7.5 14.762V6.838L1 4.239v7.923l6.5 2.6zM7.443.184a1.5 1.5 0 0 1 1.114 0l7.129 2.852A.5.5 0 0 1 16 3.5v8.662a1 1 0 0 1-.629.928l-7.185 2.874a.5.5 0 0 1-.372 0L.63 13.09a1 1 0 0 1-.63-.928V3.5a.5.5 0 0 1 .314-.464L7.443.184z"/>
        </svg>
        <span>{{ client.id }}</span>
      </div>
      <div class="col-6 node-wrapper">
        <svg class="bi bi-circle-fill" v-bind:class="client.statusColor" width="2.3rem" height="2.3rem" viewBox="0 0 16 16" fill="currentColor" xmlns="http://www.w3.org/2000/svg">
           <path fill-rule="evenodd" d="M8.5.134a1 1 0 0 0-1 0l-6 3.577a1 1 0 0 0-.5.866v6.846a1 1 0 0 0 .5.866l6 3.577a1 1 0 0 0 1 0l6-3.577a1 1 0 0 0 .5-.866V4.577a1 1 0 0 0-.5-.866L8.5.134z"/>
        </svg>
        <img src="img/loading.gif" v-bind:class="hide: client.isRunning" alt="" width="15px">
      </div>
    </div>
    <div class="row">
      <div class="col-6 node-wrapper">
        <svg class="bi bi-grid" width="2.3rem" height="2.3rem" viewBox="0 0 16 16" fill="currentColor" xmlns="http://www.w3.org/2000/svg">
          <path fill-rule="evenodd" d="M1 2.5A1.5 1.5 0 0 1 2.5 1h3A1.5 1.5 0 0 1 7 2.5v3A1.5 1.5 0 0 1 5.5 7h-3A1.5 1.5 0 0 1 1 5.5v-3zM2.5 2a.5.5 0 0 0-.5.5v3a.5.5 0 0 0 .5.5h3a.5.5 0 0 0 .5-.5v-3a.5.5 0 0 0-.5-.5h-3zm6.5.5A1.5 1.5 0 0 1 10.5 1h3A1.5 1.5 0 0 1 15 2.5v3A1.5 1.5 0 0 1 13.5 7h-3A1.5 1.5 0 0 1 9 5.5v-3zm1.5-.5a.5.5 0 0 0-.5.5v3a.5.5 0 0 0 .5.5h3a.5.5 0 0 0 .5-.5v-3a.5.5 0 0 0-.5-.5h-3zM1 10.5A1.5 1.5 0 0 1 2.5 9h3A1.5 1.5 0 0 1 7 10.5v3A1.5 1.5 0 0 1 5.5 15h-3A1.5 1.5 0 0 1 1 13.5v-3zm1.5-.5a.5.5 0 0 0-.5.5v3a.5.5 0 0 0 .5.5h3a.5.5 0 0 0 .5-.5v-3a.5.5 0 0 0-.5-.5h-3zm6.5.5A1.5 1.5 0 0 1 10.5 9h3a1.5 1.5 0 0 1 1.5 1.5v3a1.5 1.5 0 0 1-1.5 1.5h-3A1.5 1.5 0 0 1 9 13.5v-3zm1.5-.5a.5.5 0 0 0-.5.5v3a.5.5 0 0 0 .5.5h3a.5.5 0 0 0 .5-.5v-3a.5.5 0 0 0-.5-.5h-3z"/>
        </svg>
        <span>{{ client.threads }}</span>
      </div>
      <div class="col-6">
          &#960{{ client.Load }};
          <!-- &#9601; &#9602; - &#9608;-->
      </div>
    </div>
  </div>
  `
})

var app = new Vue({
  el: '#dashboard',
  data: {
    status: '...',
    statusWrapperColor: 'c11-bg c0-fg',
    isWarningsCollapsed: true,
    isErrorsCollapsed: true,
    warnings: ['[*] Cannot find config file panchaea-client.json', '[1] Thread 3 is stuck', '[1] Thread 2 is stuck'],
    warningsCount: 3,
    errors: ['[!] WorkUnit not found', '[3] Failed to reload WU'],
    errorsCount: 2,
    errorIcon: '<svg class="bi bi-asterisk c1-fg" width="1em" height="1em" viewBox="0 0 16 16" fill="currentColor" xmlns="http://www.w3.org/2000/svg"><path fill-rule="evenodd" d="M8 0a1 1 0 0 1 1 1v5.268l4.562-2.634a1 1 0 1 1 1 1.732L10 8l4.562 2.634a1 1 0 1 1-1 1.732L9 9.732V15a1 1 0 1 1-2 0V9.732l-4.562 2.634a1 1 0 1 1-1-1.732L6 8 1.438 5.366a1 1 0 0 1 1-1.732L7 6.268V1a1 1 0 0 1 1-1z"/></svg>',
    warningIcon: '<svg class="bi bi-asterisk c3-fg" width="1em" height="1em" viewBox="0 0 16 16" fill="currentColor" xmlns="http://www.w3.org/2000/svg"><path fill-rule="evenodd" d="M8 0a1 1 0 0 1 1 1v5.268l4.562-2.634a1 1 0 1 1 1 1.732L10 8l4.562 2.634a1 1 0 1 1-1 1.732L9 9.732V15a1 1 0 1 1-2 0V9.732l-4.562 2.634a1 1 0 1 1-1-1.732L6 8 1.438 5.366a1 1 0 0 1 1-1.732L7 6.268V1a1 1 0 0 1 1-1z"/></svg>',
    nodes: [{id: 0, threads: 0, status: 'ready', statusColor: 'c3-fg', load: 1, isRunning: false}]
  },
  methods: {
    isEven: function (a) {
      return a % 2 == 0
    },
    newWarning: function (warn) {
      this.warnings.push(warn)
      this.warningsCount = this.warnings.length
      Vue.toasted.show(this.warningIcon + " " + warn)
    },
    newError: function (err) {
      this.errors.push(err)
      this.errorsCount = this.errors.length
      Vue.toasted.show(this.errorIcon + " " + err)
    },
  }
})
