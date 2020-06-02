Vue.use(Toasted, {
  action : {
    text: '●',
    onClick : (e, toastObject) => {
      toastObject.goAway(0)
    },
  },
  duration: 10000,
})

// Vue.use(axios)

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
    nodes: [
      {id: 0, threads: 4, status: 'ready', statusColor: 'c3-fg', load: "&#960" + "1" + ";", isRunning: false},
      {id: 1, threads: 2, status: 'running', statusColor: 'c2-fg', load: "&#960" + "4" + ";", isRunning: true}
    ],
    workUnits: [
      {client_id: 0, thread: 1, time: "", status: "", attempt: 0}
    ]
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
    getData: function () {
      axios
      .get('api')
      .then(resp => {
	response = resp.data
	if (response.WorkUnits == null) {
		return
	}
	for (let i = 0; i < response.Warnings.length; i++) {
		this.newWarning(response.Warnings[i])
		if (i > 30) {
		  break
		}
	}
	for (let i = 0; i < response.Errors.length; i++) {
		this.newError(response.Errors[i])
		if (i > 30) {
		  break
		}
	}
        this.status = response.Status
        stColor = "c11-bg c0-fg"
	switch(this.status) {
            case 'FAILED':
              stColor = 'c9-bg c0-fg'
              break
            case 'RUNNING':
              stColor = 'c10-bg c0-fg'
              running = true
              break
            case 'READY':
              stColor = 'c11-bg c0-fg'
              break
        }
	this.statusWrapperColor = stColor
	this.nodes = []
	for (let i = 0; i < response.Clients.length; i++) {
          color = 'c3-fg'
          running = false
          switch(response.Clients[i].Status) {
            case 'ready':
              color = 'c3-fg'
              break
            case 'running':
              color = 'c2-fg'
              running = true
              break
            case 'failed':
              color = 'c1-fg'
              break
          }
          this.nodes.push({id: response.Clients[i].Id, threads: response.Clients[i].Threads, status: response.Clients[i].Status, statusColor: color, load: "&#960" + "1" + ";", isRunning: running})
        }
        /* for (let i = 0; i < response.WorkUnits.length; i++) {
          id = this.workUnits.Client.Id
          this.workUnits.push({client_id: id, thread: response.WorkUnits[i].Thread, time: "", status: response.WorkUnits[i].Status, attempt: response.WorkUnits[i].Attempt})
        }*/
      })
      .catch(error => {
      	this.status = "OFFLINE"
	this
	if (this.errors[this.errors.length - 1].message != error.message) {
	  this.newError(error.message)
	}
      })
    setTimeout(this.getData, 1000)
    }
  },
 mounted() {
     this.getData()
 }
})
