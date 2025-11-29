// @ts-nocheck
import DefaultTheme from 'vitepress/theme'

import DownloadButton from '../components/DownloadButton.vue'
import EnvInfo from '../components/EnvInfo.vue'
import LatestIssues from '../components/LatestIssues.vue'

export default {
  ...DefaultTheme,
  enhanceApp({ app }) {
    app.component('DownloadButton', DownloadButton)
    app.component('EnvInfo', EnvInfo)
    app.component('LatestIssues', LatestIssues)
  }
}

