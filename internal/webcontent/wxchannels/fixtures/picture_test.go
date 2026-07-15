package wxchannels_test

import (
	"encoding/json"
	"testing"

	wxchannels "wx_channel/internal/webcontent/wxchannels"
	scraper "wx_channel/pkg/scraper/wxchannels"
)

const pictureFeedJSON = `{
	"id": "14936583787284727824",
	"nickname": "锦妆阁汉服旅拍",
	"username": "v2_060000231003b20faec8c5eb8c1dc3d5cc02ec34b0777fde89138921d31325e189f6ac106494@finder",
	"objectDesc": {
		"description": "锦鲤上岸，事事皆如意！#锦鲤#锦妆阁汉服旅拍#杭州#锦妆阁#汉服",
		"media": [
			{
				"url": "https://finder.video.qq.com/251/20304/stodownload?encfilekey=oibeqyX228riaCwo9STVsGLPj9UYCicgttvaXDI9lCia4mLib0NZOW7iahCWzH7WFRahoyu0ckwYmqVyr0qqEEfFDaooBOt8gtfiabdw1WRAgu1W86ktGbm4V3nREzZLpRpBDG6jTXp1KqvBxo&token=2lt8WBSnjTmRlSwuic6czCg42oTPlNiaGVljGly7IA28BFayvtBOapmoWlwNhQZ7N5ejQUYiazoCmWPs8CjaR1yfda2ltujUn66cybSC7mM15vsqMWCQQKxGYUV9LFkWasPMC0j5H3MdMiazZARo4PNhwu4cQr45gLOlQZEba25Fc06bwMZRzs3bcABmyEW5vvMDInYslmWD8VAgibUvxHs7lBUIT2LibQzmgPCykicdCLfCGU&hy=SH&idx=1&m=b4c181aea3d6efdc97afea7911492ec1&uzid=1&web=1&wxampicformat=500",
				"thumbUrl": "https://finder.video.qq.com/251/20350/stodownload?encfilekey=2fG3V4WwQPkBcdYwibscMibm35dNrrGyLAlrmUGBA7cx7o1IkvjufVIcGCRogfialf4lGKKD9DYHzxur2Pr7lAGf4Hah5w06OolZ9IWiaVHIEib0&token=ic1n0xDG6aw9LibV3qFEMNRyfeT9LGxibcTLeuvpKFShjrTxbn1MpTNyFfPAg35qVRWCj71HAyqmpy8HhcefGiaHgBmXI59ibtT5jT11uUxkHmRux2jrDPTDMAkG1WJRBCOIhDg77MgH3aWQvDHN5iaB16K02YuFUuTlq4KQczbNp93kuJhhLcVlccqyyUXrj0ng94HaU2H9nsBL4ptAIZsej7W7E0WZJ2ysoR&hy=SH&idx=1&m=6204b0459a5ad2d1ddf7dca3b916c56e&picformat=200&wxampicformat=503",
				"mediaType": 2,
				"videoPlayLen": 0,
				"width": 954,
				"height": 636,
				"md5sum": "b4c181aea3d6efdc97afea7911492ec1",
				"fileSize": 0,
				"bitrate": 0,
				"spec": [],
				"coverUrl": "",
				"codecInfo": {
					"videoScore": 0,
					"videoCoverScore": 0,
					"videoAudioScore": 0,
					"thumbScore": 0,
					"hdimgScore": 0,
					"hasStickers": false,
					"useAlgorithmCover": false
				},
				"hlsSpec": {
					"hlsList": []
				},
				"hotFlag": 0,
				"halfRect": {
					"left": 0,
					"top": 151,
					"right": 954,
					"bottom": 484
				},
				"fullThumbUrl": "",
				"fullUrl": "",
				"fullWidth": 0,
				"fullHeight": 0,
				"fullFileSize": 0,
				"fullBitrate": 0,
				"fullCoverUrl": "",
				"hdrSpec": {
					"hdrList": []
				},
				"liveCoverImgs": [],
				"scalingInfo": {
					"version": "",
					"isSplitScreen": false,
					"isDisableFollow": false,
					"upPercentPosition": 0,
					"downPercentPosition": 0
				},
				"cardShowStyle": 0,
				"dynamicRangeType": 0,
				"videoType": 0,
				"duplicateFileSize": 0,
				"audioSpec": [],
				"mediaCdnInfo": {
					"isUsePcdn": true,
					"beginUsePcdnBufferSeconds": 12,
					"exitUsePcdnBufferSeconds": 8,
					"preloadBeginUsePcdnBufferKbytes": 768,
					"pcdnTimeoutRetryCount": 1,
					"marsPreDownloadKbytes": 0,
					"isUseUgcWhenNoPreload": true,
					"preloadUsePcdnOnly": true,
					"preloadPcdnConnections": 4,
					"socForceUseH3": false
				},
				"shareCoverUrl": "",
				"shareCoverShowStyle": 0,
				"cdnFileSize": 0
			},
			{
				"url": "https://finder.video.qq.com/251/20304/stodownload?encfilekey=oibeqyX228riaCwo9STVsGLPj9UYCicgttvXPRAEYq36QE9iaHSsribibGTt0icFVzuWGrhR0pm90liaHHJLtTGblXibPRRMlsEpAvZfzkCowVPDOcg0EibCo5go6w0CiaC1ZLzz5NU5Ot9icQT2rXI&token=2lt8WBSnjTmRlSwuic6czCvVJCzyIFAjhWt9W2UibFSdmBSLM3YNuJ1jXMuiaUmfyiaHibyegn0modtagkg0KUIrZxG3ialCK6UdBslsNTMiaPe4el1RDZX2TDGA6t9iaP1oiavzaowZchTGOHRhufnicyYicykyDicGTXUAMqnDM5ujJkOuH7eTUeOUEria90GLlJ7pVS08D6uBlibjuNEKe6hTcCWhGex2DkfibmkSic6aL2yR55v2Sec&hy=SH&idx=1&m=8bffac2200cf4cf93f85ab3c3480d720&uzid=1&web=1&wxampicformat=500",
				"thumbUrl": "https://finder.video.qq.com/251/20350/stodownload?encfilekey=2fG3V4WwQPlJANYWX5OG00FKn7WaYjzpgYHJohx8l5UsicIIlayymoyv2b5QYC212UzQW58iaEYdYf2qia6UEl1bGqrlRwChbL22a5rhlTnLt4&token=AxricY7RBHdW6AxiaB20ISbY3b2kln1mxWiaQAkrXibUxKzQR7Fot5QoBTpJTwxoYoribasss8G5uCVpRLicsicib6ibibbR9SGXw09sYkq4gu66MfA5aHnShtG7iaHo0TUibibggyMVkhWZb6POf9RpzB0GuZVDStPdIKj7NBm6bpWNO4AibLw9PvXdWX5JSsiakQvESdhRAnFI1hS6e8PW7uWnPoiccza18IruDfJhSNC2&hy=SH&idx=1&m=44bf5dca64aa6e96b43ec54a8091c83d&picformat=200&wxampicformat=503",
				"mediaType": 2,
				"videoPlayLen": 0,
				"width": 950,
				"height": 633,
				"md5sum": "8bffac2200cf4cf93f85ab3c3480d720",
				"fileSize": 0,
				"bitrate": 0,
				"spec": [],
				"coverUrl": "",
				"codecInfo": {
					"videoScore": 0,
					"videoCoverScore": 0,
					"videoAudioScore": 0,
					"thumbScore": 0,
					"hdimgScore": 0,
					"hasStickers": false,
					"useAlgorithmCover": false
				},
				"hlsSpec": {
					"hlsList": []
				},
				"hotFlag": 0,
				"halfRect": {
					"left": 0,
					"top": 150,
					"right": 950,
					"bottom": 482
				},
				"fullThumbUrl": "",
				"fullUrl": "",
				"fullWidth": 0,
				"fullHeight": 0,
				"fullFileSize": 0,
				"fullBitrate": 0,
				"fullCoverUrl": "",
				"hdrSpec": {
					"hdrList": []
				},
				"liveCoverImgs": [],
				"cardShowStyle": 0,
				"dynamicRangeType": 0,
				"videoType": 0,
				"duplicateFileSize": 0,
				"audioSpec": [],
				"shareCoverUrl": "",
				"shareCoverShowStyle": 0,
				"cdnFileSize": 0
			},
			{
				"url": "https://finder.video.qq.com/251/20304/stodownload?encfilekey=2fG3V4WwQPn5759PM0ZqnREUXb5xtJDCR1wtwicX3RtRegGb3Ig6WV8hxr2L7CqJgeYaDEnY7NCHczGukkia6uH78RgvGQEdMLCfZTqJmAvbE&token=ic1n0xDG6aw92aNicnLSsMnicJncBpnvExSEwkxA5K9MIsxmC6btuegAIFWRB2lt2lUxwDuq61YPl9nsianFLMRpkwmDX5S4tFicia5hGRNG9Jicjd560X6bOHpjf1Bx1NO5ITq6dZ1cZ2rhOAQbNfl2nFTYuWWibaZ0MttoXZg0433ib9VjYvLliboGZicOXzdnUdoM5ZLP7H9aXibsbbkTI8GEZkjiaCqicUvzK5walCeejDRTB8dZE&hy=SH&idx=1&m=d76daddb1793f46dc7fdf2de25322e64&uzid=1&web=1&wxampicformat=500",
				"thumbUrl": "https://finder.video.qq.com/251/20350/stodownload?encfilekey=WTva9YVXqXcSUicrMCercmDHmKYPBXC7ezG1HtDP9gHNArOd8xBf7lLOia65FFcrRM5RjSOU4uQAPBlTcuzH60TmEia7DzshVQm6RQVT7XMiaNz25ovDpibSUUP6D66OAZ94OUTorhJWOX8M&token=AxricY7RBHdUPcKCFbyPdibZEsybb0GFqfcAiaAGRQvkbnwMMVGcgibicUB5icUEn5hDKzsm9upIia4qvhKw3jRiaH5yIib5X7aTqmElwicIwIicgShXougxKibBZ9VsHLHHYBeKLrmsf85RQYO8jAo7eDRSdgCz7obNL4vUtEOHVL5iasLgyzFYDZ1vN5sl2WsC9Ys2UTmWCtWOviaHHbnj0ggxnYbrzoB7iayvzWxu2hI&hy=SH&idx=1&m=fd69b431066fd81c2a101b4813f67093&picformat=200&wxampicformat=503",
				"mediaType": 2,
				"videoPlayLen": 0,
				"width": 950,
				"height": 633,
				"md5sum": "d76daddb1793f46dc7fdf2de25322e64",
				"fileSize": 0,
				"bitrate": 0,
				"spec": [],
				"coverUrl": "",
				"codecInfo": {
					"videoScore": 0,
					"videoCoverScore": 0,
					"videoAudioScore": 0,
					"thumbScore": 0,
					"hdimgScore": 0,
					"hasStickers": false,
					"useAlgorithmCover": false
				},
				"hlsSpec": {
					"hlsList": []
				},
				"hotFlag": 0,
				"halfRect": {
					"left": 0,
					"top": 150,
					"right": 950,
					"bottom": 482
				},
				"fullThumbUrl": "",
				"fullUrl": "",
				"fullWidth": 0,
				"fullHeight": 0,
				"fullFileSize": 0,
				"fullBitrate": 0,
				"fullCoverUrl": "",
				"hdrSpec": {
					"hdrList": []
				},
				"liveCoverImgs": [],
				"cardShowStyle": 0,
				"dynamicRangeType": 0,
				"videoType": 0,
				"duplicateFileSize": 0,
				"audioSpec": [],
				"shareCoverUrl": "",
				"shareCoverShowStyle": 0,
				"cdnFileSize": 0
			},
			{
				"url": "https://finder.video.qq.com/251/20304/stodownload?encfilekey=oibeqyX228riaCwo9STVsGLPj9UYCicgttvO0HGv3yr18dFq9R3f3LPuh6xUKLj7pXSLN14r9UJRiczMvJOTauuiaYWzibO7gHQn5SN7AnWNJAiblrf7YAYJicFhx7RBtYK3BWQicTc8KoeYvDXw&token=ic1n0xDG6aw92aNicnLSsMn6k0iajibOA8Xffn2XibfRdl5ZEo6VtIuQALU9PyoW9ibdsuaWCHibI0tKArz5o5oRXqcIbJkQulAKgImlt0NoU6GOdAHCGKF4nQFgAQtMeue4ahsDB29qS7arFr09L7VEnPSLwQibbMhib6rJQy52ZDnFAXN44uicnWoxAhiclqb5GlSp9LkrsOVIADmG8jPfO0H0lrZERaFJlDA2Vk2IvNoBUA6FuM&hy=SH&idx=1&m=27a4a1457f5eee49114b6f5d89239bce&uzid=1&web=1&wxampicformat=500",
				"thumbUrl": "https://finder.video.qq.com/251/20350/stodownload?encfilekey=WTva9YVXqXcSUicrMCercmDHmKYPBXC7eKnFnkhmtjNp4icVyvFVTa1rMFOIiakjuic69aAQUJ124seg0CYU8S32KN7U0tFhwHHWZm5icEcJsSXEOVVaXCGqV2SAO4oJw3b4pPeH26MsO0AI&token=AxricY7RBHdXWjvqyGJwm0CjY6GkibSoyK2et5IYoJ4PcqMRpQUoSOFfLa85eosK8v7ibUgibNiao3ENAaExOSiaiccTicjwaPht6jryZd3YSY1rF1dW7yvc2REuZRWXZHEH2u5C7d1UcgWO1vd7h1EFeqT4ia0M1WIyOtZEG6Zh3r3Hc5s6K14XUvGN8pjHtIic0DibiaJWpVVSXE6p5ZsMaE64xaveltNQFb3gCwHp&hy=SH&idx=1&m=c47ec877c32c3c8dc2870effe1496011&picformat=200&wxampicformat=503",
				"mediaType": 2,
				"videoPlayLen": 0,
				"width": 950,
				"height": 633,
				"md5sum": "27a4a1457f5eee49114b6f5d89239bce",
				"fileSize": 0,
				"bitrate": 0,
				"spec": [],
				"coverUrl": "",
				"codecInfo": {
					"videoScore": 0,
					"videoCoverScore": 0,
					"videoAudioScore": 0,
					"thumbScore": 0,
					"hdimgScore": 0,
					"hasStickers": false,
					"useAlgorithmCover": false
				},
				"hlsSpec": {
					"hlsList": []
				},
				"hotFlag": 0,
				"halfRect": {
					"left": 0,
					"top": 150,
					"right": 950,
					"bottom": 482
				},
				"fullThumbUrl": "",
				"fullUrl": "",
				"fullWidth": 0,
				"fullHeight": 0,
				"fullFileSize": 0,
				"fullBitrate": 0,
				"fullCoverUrl": "",
				"hdrSpec": {
					"hdrList": []
				},
				"liveCoverImgs": [],
				"cardShowStyle": 0,
				"dynamicRangeType": 0,
				"videoType": 0,
				"duplicateFileSize": 0,
				"audioSpec": [],
				"shareCoverUrl": "",
				"shareCoverShowStyle": 0,
				"cdnFileSize": 0
			},
			{
				"url": "https://finder.video.qq.com/251/20304/stodownload?encfilekey=oibeqyX228riaCwo9STVsGLPj9UYCicgttvHKLMzOEOd2VZtVDCZhup4SgBumEQtdPbpJv0373bWt5ibx4qefPZYVlu4UCx6vPTicTPN9bic03aNFnD850DKeqvxoDEicaVTuiae7Gko2lKt3Do&token=ic1n0xDG6aw92aNicnLSsMn829MYgfictYroV9usFqlUZR6qEZxMMq47XUW2IV6tlSmn1RID4Y2KEdzjMTvnZG3kmK4tpZTYaVgWjhTg4hs0IEv1SAxaic0QAOctc5QwKLNsNK6VBzR03nPfxiaqzbezZqvzl4wqQR387pmCYuayKcUS9MvXfOeu13AiaZG0CM2TOib5PtOXoicGZqSGVpRynnU7p03rK1WOnJKwsVTF1st6g4Y&hy=SH&idx=1&m=5ed06cccc6dc4236f446bb5bff89dc70&uzid=1&web=1&wxampicformat=500",
				"thumbUrl": "https://finder.video.qq.com/251/20350/stodownload?encfilekey=WTva9YVXqXcSUicrMCercmDHmKYPBXC7e5EgV8scmlib4ADOqj5BuW8NshPNFjceEiaDswHN7P7JaLZcoNj2FP8hctSZ72AwcaKcvuU5BsIbFZfGWVhsFGB020T6XNJnXFe7xALBjsOpY4&token=ic1n0xDG6awicGWicviaibA5dg1GwibTqbKZWluxO5zHKq9LsjwXYrkibm0ak8Y1bsdQdicNzD2G3TPFKWPuMkIdGQBwZj7zZCytv31pzr6eEdduaNWbWvicUMP4NGlNWbnWTffCYM2AnTJpj8j7kAdAodq9Jp9POqeXaFzG1EgG50iaTsoMRqJjzhicomib9ZpCqhcTjKpPebgT8boLaeiaxh6uI1rrrTIRPAnjaOwPic&hy=SH&idx=1&m=40e26618a249f4e15ba5e2a96e468b71&picformat=200&wxampicformat=503",
				"mediaType": 2,
				"videoPlayLen": 0,
				"width": 950,
				"height": 633,
				"md5sum": "5ed06cccc6dc4236f446bb5bff89dc70",
				"fileSize": 0,
				"bitrate": 0,
				"spec": [],
				"coverUrl": "",
				"codecInfo": {
					"videoScore": 0,
					"videoCoverScore": 0,
					"videoAudioScore": 0,
					"thumbScore": 0,
					"hdimgScore": 0,
					"hasStickers": false,
					"useAlgorithmCover": false
				},
				"hlsSpec": {
					"hlsList": []
				},
				"hotFlag": 0,
				"halfRect": {
					"left": 0,
					"top": 150,
					"right": 950,
					"bottom": 482
				},
				"fullThumbUrl": "",
				"fullUrl": "",
				"fullWidth": 0,
				"fullHeight": 0,
				"fullFileSize": 0,
				"fullBitrate": 0,
				"fullCoverUrl": "",
				"hdrSpec": {
					"hdrList": []
				},
				"liveCoverImgs": [],
				"cardShowStyle": 0,
				"dynamicRangeType": 0,
				"videoType": 0,
				"duplicateFileSize": 0,
				"audioSpec": [],
				"shareCoverUrl": "",
				"shareCoverShowStyle": 0,
				"cdnFileSize": 0
			},
			{
				"url": "https://finder.video.qq.com/251/20304/stodownload?encfilekey=oibeqyX228riaCwo9STVsGLPj9UYCicgttvia0vWZGeCxLsRmmCaovwGWwdLqAVK1FGuP4aAKojiaBdrlodhN2A1P6fTibXk8myVJzTaszl54XxbnpWlFe6Bu1iaX4Wq3UqEpvTu6lnnZrfNC0&token=2lt8WBSnjTmRlSwuic6czCng4GkJELfY2lSMFmkBghgicZoia2KzYsVriaW91jG6MAxCbbibyZm1MsBgJOZH9OTIKhNr6RbRY5jvqc6HrHU8CdxVbmGP0comAQOibf6wrrNX6Y297kZbjDfNOeLCUdgjt3I3uGLUibkU8YCBwSoOP4CW9nYv1ltXEkXgwK9Lf77fS14lgouR8DKEdiaxRojriaPb3cIKITqjbKclJUAtVv7wqU0s&hy=SH&idx=1&m=65dd50a794c396df8d763298b1c48f48&uzid=1&web=1&wxampicformat=500",
				"thumbUrl": "https://finder.video.qq.com/251/20350/stodownload?encfilekey=2fG3V4WwQPlT9cAV0fPrevrUtico4fqbjhpEzobyWicebG9bibHv26z4lJgdQLKVzme5b4sw7NyPUNubQcL2TxhdbAtHr7niapFgiaEAoP3JFwMs&token=ic1n0xDG6awic67ibBVSmKne7W1dRNsW2Kc23D7aDN7siax9GdvibIGUN0B1mK6buswyFB2SFSSk9OtLjSuNibQgGrRKyfVdUibCW7hW82ChlEJAfPV6C9NSIvh9165Mia01Ua69OmPlJ5LsY4ia1mzeUgq6ZS0H6n1ODmydsamnBBzqQPfbzz4RLNgibTgIhGL91rDRpvR8B5Inp9Aq68y7jHSEr2CE2X3mOSmoCib&hy=SH&idx=1&m=f14a5139c7d79fddb1021feaef87e297&picformat=200&wxampicformat=503",
				"mediaType": 2,
				"videoPlayLen": 0,
				"width": 950,
				"height": 633,
				"md5sum": "65dd50a794c396df8d763298b1c48f48",
				"fileSize": 0,
				"bitrate": 0,
				"spec": [],
				"coverUrl": "",
				"codecInfo": {
					"videoScore": 0,
					"videoCoverScore": 0,
					"videoAudioScore": 0,
					"thumbScore": 0,
					"hdimgScore": 0,
					"hasStickers": false,
					"useAlgorithmCover": false
				},
				"hlsSpec": {
					"hlsList": []
				},
				"hotFlag": 0,
				"halfRect": {
					"left": 0,
					"top": 150,
					"right": 950,
					"bottom": 482
				},
				"fullThumbUrl": "",
				"fullUrl": "",
				"fullWidth": 0,
				"fullHeight": 0,
				"fullFileSize": 0,
				"fullBitrate": 0,
				"fullCoverUrl": "",
				"hdrSpec": {
					"hdrList": []
				},
				"liveCoverImgs": [],
				"cardShowStyle": 0,
				"dynamicRangeType": 0,
				"videoType": 0,
				"duplicateFileSize": 0,
				"audioSpec": [],
				"shareCoverUrl": "",
				"shareCoverShowStyle": 0,
				"cdnFileSize": 0
			},
			{
				"url": "https://finder.video.qq.com/251/20304/stodownload?encfilekey=2fG3V4WwQPnVzYQ7UhSUrzseUGaMDDMCrB6ahNvlJBynu8scfhiaJficEdprwBictEibBMibDgjqeOxeflkOMYxRlqUxriabE307uXtiapvdHBjaz4&token=2lt8WBSnjTmRlSwuic6czCq97cdjqxFSjhGKbb7lfRQbicLgl6RJShutwsVicjq4EjU1L46JgsyH9VBJxgl5kOzhyeNm7oZ6vtD44WUOWpdvCsTa8wIyicZLOQcowesibf9hzsMuYWbm1eHGx5pfSm32Ciaacf1uibqE1BQGF0LgBPrneqV4WpTqXOGz6kLBD8YmHWiaibxtGnxU7QlA3BrhjWFUXNngTHl1PEhCld3gSUaferGI&hy=SH&idx=1&m=77992e37bc596843e3d507889c73cf5b&uzid=1&web=1&wxampicformat=500",
				"thumbUrl": "https://finder.video.qq.com/251/20350/stodownload?encfilekey=WTva9YVXqXcSUicrMCercmDHmKYPBXC7e4BN5RISBwQJtcdzGCOO9C65C84zplO2ueOy3Yjf2NxMlKaIiasxwo9sAcv3rh7R8StVibl4ywBlykQwMOyHicu1yq9aQnog54O7ibt3hyT4iaicS0&token=AxricY7RBHdXNjDsjTicJShMsic9niblsSQnIPMiaCcqPw3S67LNvPicHozcvovMTn2ye2VqLbgdqDOia4LDsoAx3lSnLpILoiawlGJka6aOvTGZrTHPQsvnpA1BickoBDiaWWA1Y9kAzibbiaibmM76gZ3bRia7x0RsISTD3BVUdQ1zBjDN2pl07c6vzf7rDDmXA6LhTwTWsS2pDFSv1pDBvY9G5pHqrthblMMQYRibA2l&hy=SH&idx=1&m=a61638ff51c1161119325bdcad45ce80&picformat=200&wxampicformat=503",
				"mediaType": 2,
				"videoPlayLen": 0,
				"width": 1055,
				"height": 1584,
				"md5sum": "77992e37bc596843e3d507889c73cf5b",
				"fileSize": 0,
				"bitrate": 0,
				"spec": [],
				"coverUrl": "",
				"codecInfo": {
					"videoScore": 0,
					"videoCoverScore": 0,
					"videoAudioScore": 0,
					"thumbScore": 0,
					"hdimgScore": 0,
					"hasStickers": false,
					"useAlgorithmCover": false
				},
				"hlsSpec": {
					"hlsList": []
				},
				"hotFlag": 0,
				"halfRect": {
					"left": 0,
					"top": 376,
					"right": 1055,
					"bottom": 1207
				},
				"fullThumbUrl": "",
				"fullUrl": "",
				"fullWidth": 0,
				"fullHeight": 0,
				"fullFileSize": 0,
				"fullBitrate": 0,
				"fullCoverUrl": "",
				"hdrSpec": {
					"hdrList": []
				},
				"liveCoverImgs": [],
				"cardShowStyle": 0,
				"dynamicRangeType": 0,
				"videoType": 0,
				"duplicateFileSize": 0,
				"audioSpec": [],
				"shareCoverUrl": "",
				"shareCoverShowStyle": 0,
				"cdnFileSize": 0
			},
			{
				"url": "https://finder.video.qq.com/251/20304/stodownload?encfilekey=oibeqyX228riaCwo9STVsGLPj9UYCicgttvpb5BumObWl2akTE7O7gLzhY96FGxhKJlEhibgeibPRP9F9vkUURaANwOTXjT7SjrbcIcHVg8ic77gMIL0tDMJ0a4TA5zXZichWLCz4Gvwmz6yzw&token=2lt8WBSnjTmRlSwuic6czCs3ia2ibLfbxMd7TNjzTibUYL5iarRNibMbGzWrQoPbDFRPIJn80FBahz2Bu37FtdjmX8ibZHP025cPibHwrIUfXZiaMlEBWhsq5Mm5DxKUeShBVceyjh3o8ZGrUWrq3uHicBbYicOrY63ic9Pu7FjvAhdDZLwQr03Y0VSz5DhH04C2KZ5xXiaRtL4CYqFcHWdVP7nvx1TIabtOUVwbZbvnoHWDwV2uvj1w&hy=SH&idx=1&m=8c3176643bab0e59290ef88d3c420750&uzid=1&web=1&wxampicformat=500",
				"thumbUrl": "https://finder.video.qq.com/251/20350/stodownload?encfilekey=2fG3V4WwQPl9Zld3leXU5xJE0GjibO9t3UtRia0SDsvJUTPZ87zc3NxknoHCqlVST6Uag6nCmibsghKicA7ZfDrIuqlGJq18cmFjr1Ria1gNicZVQ&token=ic1n0xDG6aw9tr7oZKPnPaAXasnc9EUjS9iahf2yrxHVOPjnL8lFMHYzAhicn1KcY5bOBzxfz75N3gjOIuwfx5qDEChTicKhvqpM6C4vUlKWNlH9yeib7RZDOWuMzIoGQJl66eBDb1NPvzibjReQJ2LibqsLyjqibeXCNLcAaEbC60tJypy1oZdZI2qxBfHxzo8EpbPKibt4D382KUQ824y5OH4YkQ5vLlzYQw4RH&hy=SH&idx=1&m=e962b8d81c03dd40c3e321a662ad9391&picformat=200&wxampicformat=503",
				"mediaType": 2,
				"videoPlayLen": 0,
				"width": 1055,
				"height": 1584,
				"md5sum": "8c3176643bab0e59290ef88d3c420750",
				"fileSize": 0,
				"bitrate": 0,
				"spec": [],
				"coverUrl": "",
				"codecInfo": {
					"videoScore": 0,
					"videoCoverScore": 0,
					"videoAudioScore": 0,
					"thumbScore": 0,
					"hdimgScore": 0,
					"hasStickers": false,
					"useAlgorithmCover": false
				},
				"hlsSpec": {
					"hlsList": []
				},
				"hotFlag": 0,
				"halfRect": {
					"left": 0,
					"top": 376,
					"right": 1055,
					"bottom": 1207
				},
				"fullThumbUrl": "",
				"fullUrl": "",
				"fullWidth": 0,
				"fullHeight": 0,
				"fullFileSize": 0,
				"fullBitrate": 0,
				"fullCoverUrl": "",
				"hdrSpec": {
					"hdrList": []
				},
				"liveCoverImgs": [],
				"cardShowStyle": 0,
				"dynamicRangeType": 0,
				"videoType": 0,
				"duplicateFileSize": 0,
				"audioSpec": [],
				"shareCoverUrl": "",
				"shareCoverShowStyle": 0,
				"cdnFileSize": 0
			},
			{
				"url": "https://finder.video.qq.com/251/20304/stodownload?encfilekey=2fG3V4WwQPnUxFu8PZrscxBu4IZ4f1theuGibJZtKmQLoD3IiahbDBkRPCJZAJs0u82aOe1ks5gYS9QxwYs23DAgU1huB46GGrnBMeRYNIbh4&token=2lt8WBSnjTmRlSwuic6czClKPggFm9cql9tbgje0RfTgUFvQXJAQxtObVibtNpd6JZA3Y6r8gX1qQibI2nBcusYPWRxgHz5ZFRHWaVmdfRsUrzyw7teGILbrUdcHGC1vBicvhkhyo5SJjv97xV7qafxvm8MVYatJeZ3ULcURDAWneibH84xVEb7a7r4xO0uCQzOwMsBVtOVSJc0UZibGc6n49oW1UbibJJazFft76icEHBF1Ueg&hy=SH&idx=1&m=d536fda4e37bf3fa105fd25b1c95972e&uzid=1&web=1&wxampicformat=500",
				"thumbUrl": "https://finder.video.qq.com/251/20350/stodownload?encfilekey=WTva9YVXqXcSUicrMCercmDHmKYPBXC7eq8LWiaF8x8EMOS2L1tkDhcEJVzAa3jvMicKVWzAg6OYoBWsIBAKiblJQSWI09m4BmubibITMBYYZnupw3MicvKdtPe29XQnnWkDSicLrElu7nuyg0&token=AxricY7RBHdXgdqgwC439vza0ZozKPH4XTalttSGEbCkPULCQ3Vich5lprhlmlNr6Xtk6ufibwnicrMoeictOdUfice8xkmX187Yan0QVM1dpwJRY3VPAqxhM1SOQT7EBSWErTEHIBQpJk8ZTm0wvs3iacicSr3FfayLWzX9fic2lp8ZyZ7yMoPpUc8VXmpl9JjS49135V0mhiabvZry6ibkicuQcAGjicVm51cxkbnSV&hy=SH&idx=1&m=b68f13cbe67cbbdc5135d06750fd8ffa&picformat=200&wxampicformat=503",
				"mediaType": 2,
				"videoPlayLen": 0,
				"width": 950,
				"height": 633,
				"md5sum": "d536fda4e37bf3fa105fd25b1c95972e",
				"fileSize": 0,
				"bitrate": 0,
				"spec": [],
				"coverUrl": "",
				"codecInfo": {
					"videoScore": 0,
					"videoCoverScore": 0,
					"videoAudioScore": 0,
					"thumbScore": 0,
					"hdimgScore": 0,
					"hasStickers": false,
					"useAlgorithmCover": false
				},
				"hlsSpec": {
					"hlsList": []
				},
				"hotFlag": 0,
				"halfRect": {
					"left": 0,
					"top": 150,
					"right": 950,
					"bottom": 482
				},
				"fullThumbUrl": "",
				"fullUrl": "",
				"fullWidth": 0,
				"fullHeight": 0,
				"fullFileSize": 0,
				"fullBitrate": 0,
				"fullCoverUrl": "",
				"hdrSpec": {
					"hdrList": []
				},
				"liveCoverImgs": [],
				"cardShowStyle": 0,
				"dynamicRangeType": 0,
				"videoType": 0,
				"duplicateFileSize": 0,
				"audioSpec": [],
				"shareCoverUrl": "",
				"shareCoverShowStyle": 0,
				"cdnFileSize": 0
			}
		],
		"mediaType": 2,
		"extra": {
			"coverUrlWord": [],
			"isRealshoot": 0,
			"shareCoverUrlWord": []
		},
		"location": {
			"longitude": 120.1500015258789,
			"latitude": 30.25,
			"city": "杭州市",
			"poiName": "锦妆阁汉服旅拍",
			"poiAddress": "浙江省杭州市上城区湖滨街道长生路58号西湖国贸中心424室",
			"poiClassifyId": "qqmap_7624856631103364244",
			"poiClassifyType": 0,
			"country": "中国",
			"source": 0,
			"flag": 0,
			"productId": [],
			"commercializationFlag": 0,
			"multiLangInfo": [],
			"countryCode": "CN",
			"adcode": 330102
		},
		"extReading": {
			"type": 0,
			"style": 0
		},
		"topic": {},
		"mentionedUser": [],
		"feedLocation": {
			"productId": [],
			"multiLangInfo": []
		},
		"mentionedMusics": [],
		"imgFeedBgmInfo": {
			"docId": "78281156242757032",
			"albumThumbUrl": "http://wx.y.gtimg.cn/music/photo_new/T002R500x500M000001wYr5j06lYrD_3.jpg",
			"name": "洛春赋",
			"artist": "云汐",
			"mediaStreamingUrl": "http://wx.music.tc.qq.com/RS0400168uEl18VckC.mp3?guid=2000000354&vkey=CF675425D6541B275D931A80946FCB48E4DC03D94BBA3EE0C0408CA8DBD06BC6DB11A4C450B1BBF43AFC6886EDD10ED1755142CC2268A9B6__v2157bb3d6&uin=0&fromtag=99010354&trace=0419e894fdfd3201",
			"musicPlayLen": 0,
			"docType": 1,
			"isTrySong": 0
		},
		"followPostInfo": {
			"musicInfo": {
				"docId": "78281156242757032",
				"albumThumbUrl": "http://wx.y.gtimg.cn/music/photo_new/T002R500x500M000001wYr5j06lYrD_3.jpg",
				"name": "洛春赋",
				"artist": "云汐",
				"mediaStreamingUrl": "http://wx.music.tc.qq.com/RS0400168uEl18VckC.mp3?guid=2000000354&vkey=CF675425D6541B275D931A80946FCB48E4DC03D94BBA3EE0C0408CA8DBD06BC6DB11A4C450B1BBF43AFC6886EDD10ED1755142CC2268A9B6__v2157bb3d6&uin=0&fromtag=99010354&trace=0419e894fdfd3201",
				"musicPlayLen": 0,
				"docType": 1,
				"isTrySong": 0,
				"transferInfo": {
					"listenId": 78281156242757020,
					"songId": 402371058,
					"beginMs": 0,
					"endMs": 191000,
					"transferTime": 1771735485,
					"finderUrl": "http://wxapp.tc.qq.com/251/20305/stodownload?bizid=1023&dotrans=0&filekey=30250201010411300f020200fb040253480400020346be3d040d00000004627466730000000132&hy=SH&m=&storeid=5699a89bc000dee970e7db156000000fb00004f5153480150b1b156b2af86d&uzid=1"
				}
			},
			"groupId": "Listen_78281156242757032",
			"hasBgm": 1
		},
		"fromApp": {
			"appid": "",
			"uiStyle": "0",
			"extInfo": "",
			"source": 0,
			"sdkExtInfo": ""
		},
		"event": {
			"eventTopicId": "0",
			"eventName": "",
			"eventCreatorNickname": "",
			"eventAttendCount": 0,
			"hiddenMark": 0,
			"feedCount": 0,
			"isNeedPreload": 0
		},
		"draftObjectId": "0",
		"clientDraftExtInfo": {
			"waitType": 0,
			"coverTimeStamp": 0,
			"coverWordInfo": [],
			"needPostATemplateComment": 0,
			"memberData": {
				"postWithMemberZoneLink": 0
			},
			"videoSourceType": 0,
			"feedLongitude": 0,
			"feedLatitude": 0,
			"sourceEnterScene": 0,
			"shootMusicReportInfo": {
				"type": 0,
				"scene": 1,
				"sourceScene": 0,
				"isAttachMusic": 0,
				"isAttachLyric": 0,
				"isCloseSound": 0,
				"bgmPanelIndex": 0,
				"selectType": 0,
				"posId": 0
			},
			"editMusicReportInfo": {
				"type": 0,
				"scene": 2,
				"sourceScene": 0,
				"isAttachMusic": 0,
				"isAttachLyric": 0,
				"isCloseSound": 0,
				"bgmPanelIndex": 0,
				"selectType": 0,
				"posId": 0
			},
			"coverSelectSource": 0
		},
		"generalReportInfo": {
			"clientInfo": "eyJlbnRlcnNjZW5lIjoyLCJ2aWRlb3NvdXJjZSI6MSwiY2hpbGRfZW50ZXJzY2VuZSI6MCwiY29tbWVudFNjZW5lIjozM30="
		},
		"posterLocation": {
			"city": "Hangzhou City",
			"productId": [],
			"multiLangInfo": [],
			"adcode": 330100
		},
		"shortTitle": [],
		"flowCardDesc": {
			"description": "锦鲤上岸，事事皆如意！"
		},
		"finderNewlifeDesc": {
			"secretlyPushChatroomName": [],
			"commentEggInfo": [],
			"pictureCutRatioForFinder": 1.3333333333333333,
			"videoTmplInfo": [],
			"customCropInfo": [],
			"mpLocations": []
		},
		"memberData": {
			"postWithMemberZoneLink": 0
		},
		"modFeedInfo": {
			"history": [],
			"modifyButtonStatus": 0
		},
		"publisherVideoInfo": {
			"multiEditingTools": [],
			"videoSource": 1,
			"showWording": "未上报来源"
		},
		"setInteractionEasterEggScene": 0
	},
	"createtime": 1780579541,
	"likeFlag": 0,
	"likeList": [],
	"commentList": [],
	"forwardCount": 1,
	"contact": {
		"username": "v2_060000231003b20faec8c5eb8c1dc3d5cc02ec34b0777fde89138921d31325e189f6ac106494@finder",
		"nickname": "锦妆阁汉服旅拍",
		"headUrl": "https://wx.qlogo.cn/finderhead/ver_1/ZM5cGIPN3utxib2f1glenhaokUeb1iaIe2sRDW3QQnmXf374hJLTu5z0CicmBuHFtbKpgKnWQ6NWDHY01KuRGwyUEo6mrqYcGaeV00Pd2NAH5ibEDG7E44j1bnAkLgKxkQHk559VEVmkhbvgFvpR8VKGng/132",
		"signature": "📍杭州西湖国贸中心424 锦妆阁\n️19560433888\n🫘锦妆阁官方号（杭州西湖店）\n🍠锦妆阁汉服旅拍（杭州西湖店）\n预约可加V\n其他暂未入驻～",
		"followFlag": 0,
		"coverImgUrl": "",
		"spamStatus": 0,
		"extFlag": 262148,
		"extInfo": {
			"country": "CN",
			"province": "Zhejiang",
			"city": "Hangzhou"
		},
		"liveStatus": 2,
		"liveCoverImgUrl": "",
		"liveInfo": {
			"anchorStatusFlag": "2048",
			"switchFlag": 4607,
			"sourceType": 0,
			"micSetting": {
				"settingFlag": 0,
				"settingSwitchFlag": 4
			},
			"lotterySetting": {},
			"liveCoverImgs": [],
			"replaySetting": {
				"canUseIntelligentlyGenReplayHighlight": true
			}
		},
		"friendFollowCount": 0,
		"feedCount": 78,
		"bindInfo": [],
		"menu": [],
		"status": "0",
		"additionalFlag": "1025",
		"referenceInfo": [
			{
				"type": 1,
				"name": "公众号/服务号",
				"status": 1
			},
			{
				"type": 2,
				"name": "小程序",
				"status": 1
			},
			{
				"type": 4,
				"name": "秒剪",
				"status": 2
			}
		]
	},
	"recommenderList": [],
	"likeCount": 1,
	"commentCount": 0,
	"friendLikeCount": 0,
	"objectNonceId": "8189655580179332358_0_146_0_0",
	"objectStatus": 0,
	"sendShareFavWording": "",
	"originalFlag": 0,
	"secondaryShowFlag": 1,
	"mentionedUserContact": [],
	"sessionBuffer": "eyJjdXJfbGlrZV9jb3VudCI6MSwicmVjYWxsX3R5cGVzIjpbXSwiZGVsaXZlcnlfc2NlbmUiOjYsImRlbGl2ZXJ5X3RpbWUiOjE3ODM2OTMyMDMsInNldF9jb25kaXRpb25fZmxhZyI6MjksInJlY2FsbF9pbmRleCI6W10sInJlcXVlc3RfaWQiOjUwMTYzNDU2MzUzNTE2MzQsInJlY2FsbF9pbmZvIjpbXSwic2VjcmV0ZV9kYXRhIjoiQmdBQVBRTHZIenNGdVZxY0tDT0QzVEpCcFJDc3c4RmxvaTZhMDBmYlJqS3FJbWd2Q1VzUXhJTHhTeEQ0dThGbVk0d0lMdEFWdDIwZCIsImlkYyI6MywiZGV2aWNlX3R5cGVfaWQiOjI5LCJwdWxsX3R5cGUiOjQsImNsaWVudF9yZXBvcnRfYnVmZiI6IntcImVudHJhbmNlSWRcIjpcIjEwMDJcIn0iLCJjb21tZW50X3NjZW5lIjoxNDAsIm9iamVjdF9pZCI6MTQ5MzY1ODM3ODcyODQ3Mjc4MjQsImV4cHRfZmxhZyI6MSwiZXJpbCI6W10sInBna2V5cyI6W10sInNjaWQiOiI3MTQ0ZGQxYS03YzZhLTExZjEtYTM5Yi00NTcyOGJlOTQyMzUifQ==",
	"favCount": 1,
	"favFlag": 0,
	"urlValidDuration": 172800,
	"forwardStyle": 0,
	"permissionFlag": 2147483656,
	"objectType": 0,
	"friendCommentList": [],
	"adFlag": 4,
	"funcFlag": 272,
	"showOriginal": false,
	"finderPromotionJumpinfo": {
		"jumpInfo": {
			"jumpinfoType": 1,
			"wording": "帮上热门",
			"miniAppInfo": {
				"appId": "wx0ebcb2fd0155584d",
				"path": "pages/promote/PromoteFinderForm.html",
				"extraData": "eyJleHBvcnRfaWQiOiJleHBvcnQvVXpGZkJnQUF4TVNqTEVrQ0ZnUEJqTXpUNERDTGg3U0xkZTdSUVlXUUI2aktlT1BxX1EifQ=="
			},
			"style": [],
			"supportDeviceList": []
		},
		"wording": "帮上热门",
		"destinationType": 1
	},
	"ipRegionInfo": {
		"regionText": "Zhejiang"
	},
	"objectExtend": {
		"favInfo": {
			"starFavFlag": 0,
			"starFavCount": 0,
			"fingerlikeFavFlag": 0,
			"fingerlikeFavCount": 1
		},
		"preloadConfig": {
			"commentIsPreload": true,
			"commentWaitTime": 0,
			"commentPreloadBuffer": "CAEQAA=="
		},
		"monotonicData": {
			"countInfo": {
				"commentCount": 0,
				"likeCount": 1,
				"forwardCount": 1,
				"readCount": 0,
				"favCount": 1,
				"versionData": {
					"dataVersion": 1783086965,
					"overwrite": false
				}
			},
			"commentCount": {
				"commentCount": 0,
				"imageCommentCount": 0,
				"versionData": {
					"dataVersion": 1783086965
				}
			},
			"globalFavCount": {},
			"globalFavFlag": {},
			"thumbUpCount": {
				"thumbUpCount": 1,
				"versionData": {
					"dataVersion": 0
				}
			},
			"thumbUpFlag": {},
			"chatroomPushCount": {},
			"chatroomPushFlag": {
				"chatroomPushList": []
			},
			"thankChatroomPushFlag": {}
		},
		"finderNewlifeInfo": {
			"chatroomPushList": [],
			"pictureCropInfo": [],
			"followPostInfo": {}
		},
		"originalInfo": {
			"originalAuditStatus": 1
		},
		"carouselInfo": {
			"carouselCommentLatencyTime": 10
		},
		"streamContextId": "7144dd1a-7c6a-11f1-a39b-45728be94235"
	}
}
`

func TestToAccount_FromPictureFeedJSON(t *testing.T) {
	var obj scraper.ChannelsObject
	if err := json.Unmarshal([]byte(pictureFeedJSON), &obj); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	account, err := wxchannels.ToAccount(&obj)
	if err != nil {
		t.Fatalf("ToAccount: %v", err)
	}

	expectedUsername := "v2_060000231003b20faec8c5eb8c1dc3d5cc02ec34b0777fde89138921d31325e189f6ac106494@finder"
	if account.ExternalId != expectedUsername {
		t.Errorf("ExternalId = %q", account.ExternalId)
	}
	if account.Nickname != "锦妆阁汉服旅拍" {
		t.Errorf("Nickname = %q, want %q", account.Nickname, "锦妆阁汉服旅拍")
	}
	if account.PlatformId != "wx_channels" {
		t.Errorf("PlatformId = %q", account.PlatformId)
	}
	_ = expectedUsername // account.Id is auto-increment int
}

func TestToContent_FromPictureFeedJSON(t *testing.T) {
	var obj scraper.ChannelsObject
	if err := json.Unmarshal([]byte(pictureFeedJSON), &obj); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	content, err := wxchannels.ToContent(&obj)
	if err != nil {
		t.Fatalf("ToContent: %v", err)
	}

	if content.ContentType != "picture" {
		t.Errorf("ContentType = %q, want %q", content.ContentType, "picture")
	}
	if content.Title != "锦鲤上岸，事事皆如意！#锦鲤#锦妆阁汉服旅拍#杭州#锦妆阁#汉服" {
		t.Errorf("Title = %q", content.Title)
	}
	if content.Description != content.Title {
		t.Errorf("Description = %q, want match Title", content.Description)
	}
	if content.ExternalId != "14936583787284727824" {
		t.Errorf("ExternalId = %q", content.ExternalId)
	}
	if content.ExternalId2 != "8189655580179332358_0_146_0_0" {
		t.Errorf("ExternalId2 = %q, want %q", content.ExternalId2, "8189655580179332358_0_146_0_0")
	}
	if content.ExternalId3 != "" {
		t.Errorf("ExternalId3 = %q, want empty", content.ExternalId3)
	}
	_ = content.ContentType // content.Id is auto-increment int
	if content.ContentURL != "" {
		t.Errorf("ContentURL = %q, want empty", content.ContentURL)
	}
	if content.URL != "" {
		t.Errorf("URL = %q, want empty", content.URL)
	}
	if content.CoverURL != "" {
		t.Errorf("CoverURL = %q, want empty (picture media has empty coverUrl)", content.CoverURL)
	}
	if content.SourceURL != "" {
		t.Errorf("SourceURL = %q, want empty", content.SourceURL)
	}
	if content.PublishTime == nil || *content.PublishTime != 1780579541 {
		t.Errorf("PublishTime = %v, want ptr to 1780579541", content.PublishTime)
	}
	if content.Duration != 0 {
		t.Errorf("Duration = %d, want 0", content.Duration)
	}
	if content.Size != 0 {
		t.Errorf("Size = %d, want 0", content.Size)
	}
	expectedMetadata := `{"key":""}`
	if content.Metadata != expectedMetadata {
		t.Errorf("Metadata = %q, want %q", content.Metadata, expectedMetadata)
	}
}
