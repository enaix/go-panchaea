Vue.use(Toasted, {
  action : {
    text: '●',
    onClick : (e, toastObject) => {
      toastObject.goAway(0)
    },
  },
  duration: 10000,
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
