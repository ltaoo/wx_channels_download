// @ts-nocheck
import DefaultTheme from 'vitepress/theme'

import CustomHomeHero from '../components/CustomHomeHero.vue'
import DownloadButton from '../components/DownloadButton.vue'
import DownloadButtons from '../components/DownloadButtons.vue'
import EnvInfo from '../components/EnvInfo.vue'
import LatestIssues from '../components/LatestIssues.vue'
import ScalarApiReference from './components/ScalarApiReference.vue'
import SponsorList from '../components/SponsorList.vue'

export default {
  ...DefaultTheme,
  enhanceApp({ app }) {
    app.component('CustomHomeHero', CustomHomeHero)
    app.component('DownloadButton', DownloadButton)
    app.component('DownloadButtons', DownloadButtons)
    app.component('EnvInfo', EnvInfo)
    app.component('LatestIssues', LatestIssues)
    app.component('ScalarApiReference', ScalarApiReference)
    app.component('SponsorList', SponsorList)
  }
}

