<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { gsap } from 'gsap'
import { ScrollTrigger } from 'gsap/ScrollTrigger'

gsap.registerPlugin(ScrollTrigger)

const props = defineProps({
  siteTitle: {
    type: String,
    default: '星空AI'
  },
  plans: {
    type: Array,
    default: () => []
  },
  allowRegistration: {
    type: Boolean,
    default: true
  }
})

const emit = defineEmits(['navigate', 'start'])

const rootRef = ref(null)
const activeSlide = ref(0)
let mm = null
let ctx = null

const slides = [
  {
    kicker: 'AI Gateway for Builders',
    title: '星空 AI',
    headline: '把顶级模型接入你的工作流',
    text: '面向中国开发者的一站式 AI API 服务。低成本额度、亚洲节点中转、统一模型接口和清晰控制台，让产品、脚本和团队工具稳定跑起来。',
    accent: 'mint',
    chips: ['GPT', 'Claude', 'OpenAI API']
  },
  {
    kicker: 'Low Cost API',
    title: '低成本',
    headline: '每一次调用都清楚可控',
    text: '额度、套餐、价格和用量记录收束在同一个体验里。先小额度试跑，再按业务频率扩展。',
    accent: 'gold',
    chips: ['日套餐', '周套餐', '公共额度']
  },
  {
    kicker: 'Asia Relay',
    title: '快响应',
    headline: '更贴近中文开发场景',
    text: '亚洲节点中转减少跨区波动，适合调试、批量生成、自动化脚本和产品内嵌 AI 功能。',
    accent: 'blue',
    chips: ['低延迟', '稳定中转', '渠道监控']
  },
  {
    kicker: 'One Console',
    title: '全掌控',
    headline: 'Key、订单、日志一个地方完成',
    text: '购买套餐、创建密钥、查看请求记录和响应耗时都在控制台里完成，不需要在多个系统之间切换。',
    accent: 'pink',
    chips: ['API Key', '使用记录', '模型价格']
  }
]

const currentSlide = computed(() => slides[activeSlide.value] || slides[0])

onMounted(async () => {
  await nextTick()
  setupScrollExperience()
})

onBeforeUnmount(() => {
  mm?.revert()
  ctx?.revert()
  mm = null
  ctx = null
})

function setupScrollExperience() {
  if (!rootRef.value) return

  ctx = gsap.context(() => {
    mm = gsap.matchMedia()

    mm.add(
      {
        desktop: '(min-width: 900px)',
        reduceMotion: '(prefers-reduced-motion: reduce)'
      },
      (context) => {
        const { desktop, reduceMotion } = context.conditions
        const panels = gsap.utils.toArray('.fullscreen-copy-panel')
        const visuals = gsap.utils.toArray('.fullscreen-visual-layer')

        if (reduceMotion || !desktop) {
          gsap.set([...panels, ...visuals], { clearProps: 'all' })
          return
        }

        gsap.set(panels, { autoAlpha: 0, y: 90, scale: 0.92 })
        gsap.set(visuals, { autoAlpha: 0, scale: 1.12, rotation: -2 })
        gsap.set(panels[0], { autoAlpha: 1, y: 0, scale: 1 })
        gsap.set(visuals[0], { autoAlpha: 1, scale: 1, rotation: 0 })

        const tl = gsap.timeline({
          defaults: { ease: 'none' },
          scrollTrigger: {
            trigger: '.fullscreen-pin',
            start: 'top top',
            end: `+=${slides.length * 920}`,
            scrub: 0.78,
            pin: true,
            anticipatePin: 1,
            snap: {
              snapTo: 1 / (slides.length - 1),
              duration: 0.34,
              delay: 0.04,
              ease: 'power2.inOut'
            },
            onUpdate: (self) => {
              activeSlide.value = Math.min(slides.length - 1, Math.round(self.progress * (slides.length - 1)))
            }
          }
        })

        slides.forEach((_, index) => {
          if (index === 0) return
          const prev = index - 1
          tl.to(panels[prev], { autoAlpha: 0, y: -92, scale: 0.94, duration: 0.5 }, index - 0.32)
            .to(visuals[prev], { autoAlpha: 0, scale: 0.9, rotation: 2, duration: 0.5 }, '<')
            .to(panels[index], { autoAlpha: 1, y: 0, scale: 1, duration: 0.62 }, index - 0.12)
            .to(visuals[index], { autoAlpha: 1, scale: 1, rotation: 0, duration: 0.62 }, '<')
            .to('.ambient-ring', { rotate: index * 48, scale: 1 + index * 0.06, duration: 0.72 }, '<')
            .to('.stage-background', { '--stage-shift': `${index * 24}%`, duration: 0.72 }, '<')
        })

        gsap.from('.fullscreen-copy-panel:first-child .topline-reveal', {
          y: 30,
          autoAlpha: 0,
          duration: 0.9,
          ease: 'power3.out',
          stagger: 0.08
        })
      },
      rootRef.value
    )
  }, rootRef.value)
}
</script>

<template>
  <main ref="rootRef" class="home-fullscreen">
    <section class="fullscreen-pin">
      <div class="stage-background" :class="`accent-${currentSlide.accent}`"></div>
      <div class="stage-vignette"></div>
      <div class="ambient-ring" aria-hidden="true"></div>

      <div
        v-for="(slide, index) in slides"
        :key="`${slide.kicker}-visual`"
        class="fullscreen-visual-layer"
        :class="`accent-${slide.accent}`"
        aria-hidden="true"
      >
        <div class="ambient-aurora aurora-a"></div>
        <div class="ambient-aurora aurora-b"></div>
        <div class="ambient-aurora aurora-c"></div>
        <div class="ambient-line line-a"></div>
        <div class="ambient-line line-b"></div>
        <div class="ambient-grid"></div>
      </div>

      <div class="fullscreen-copy">
        <article
          v-for="(slide, index) in slides"
          :key="slide.kicker"
          class="fullscreen-copy-panel mx-auto max-w-7xl px-4 sm:px-6"
          :class="{ active: activeSlide === index }"
        >
          <p class="topline-reveal stage-kicker">{{ slide.kicker }}</p>
          <h1 v-if="index === 0" class="topline-reveal">{{ slide.title }}</h1>
          <h2 v-else class="topline-reveal">{{ slide.title }}</h2>
          <strong class="topline-reveal">{{ slide.headline }}</strong>
          <span class="topline-reveal">{{ slide.text }}</span>
          <div v-if="index === 0" class="fullscreen-actions topline-reveal">
            <button class="stage-primary" type="button" @click="emit('start')">
              {{ allowRegistration ? '立即开始' : '登录使用' }}
              <el-icon><ArrowRight /></el-icon>
            </button>
            <button class="stage-secondary" type="button" @click="emit('navigate', '/models')">
              查看模型
              <el-icon><Connection /></el-icon>
            </button>
          </div>
          <div class="stage-facts topline-reveal">
            <span><b>统一</b> API 接口</span>
            <span><b>稳定</b> 亚洲中转</span>
            <span><b>透明</b> 用量记录</span>
          </div>
        </article>
      </div>

      <div class="stage-progress" aria-hidden="true">
        <span
          v-for="(_, index) in slides"
          :key="index"
          :class="{ active: activeSlide === index }"
        ></span>
      </div>
      <div class="scroll-cue" aria-hidden="true">
        <span>Scroll</span>
        <i></i>
      </div>
    </section>

    <section class="fullscreen-final">
      <div class="mx-auto max-w-7xl px-4 sm:px-6">
        <p>Ready</p>
        <h2>一个 API Key，开始接入模型能力。</h2>
        <div class="fullscreen-actions">
          <button class="stage-primary" type="button" @click="emit('start')">
            进入控制台
            <el-icon><ArrowRight /></el-icon>
          </button>
          <button class="stage-secondary" type="button" @click="emit('navigate', '/plans')">
            查看价格
            <el-icon><Money /></el-icon>
          </button>
        </div>
      </div>
    </section>
  </main>
</template>

<style scoped>
.home-fullscreen {
  overflow: hidden;
  background: #050914;
  color: #f8fafc;
}

.fullscreen-pin {
  position: relative;
  display: grid;
  min-height: calc(100vh - 72px);
  overflow: hidden;
  isolation: isolate;
}

.stage-background {
  --stage-shift: 0%;
  position: absolute;
  inset: 0;
  z-index: -5;
  background:
    radial-gradient(circle at calc(50% + var(--stage-shift)) 26%, var(--accent-soft), transparent 28%),
    radial-gradient(circle at calc(52% - var(--stage-shift)) 72%, rgba(45, 212, 191, 0.14), transparent 30%),
    linear-gradient(180deg, #070a13 0%, #020610 72%, #050914 100%);
}

.stage-vignette {
  position: absolute;
  inset: 0;
  z-index: -2;
  background:
    radial-gradient(ellipse at center, transparent 26%, rgba(2, 6, 16, 0.8) 100%),
    linear-gradient(180deg, rgba(2, 6, 16, 0.08), rgba(2, 6, 16, 0.38));
}

.accent-mint {
  --accent-main: #a7f3d0;
  --accent-soft: rgba(52, 211, 153, 0.24);
  --accent-line: rgba(125, 211, 252, 0.6);
}

.accent-gold {
  --accent-main: #fde68a;
  --accent-soft: rgba(251, 191, 36, 0.24);
  --accent-line: rgba(253, 230, 138, 0.62);
}

.accent-blue {
  --accent-main: #93c5fd;
  --accent-soft: rgba(59, 130, 246, 0.25);
  --accent-line: rgba(147, 197, 253, 0.62);
}

.accent-pink {
  --accent-main: #f0abfc;
  --accent-soft: rgba(217, 70, 239, 0.23);
  --accent-line: rgba(240, 171, 252, 0.62);
}

.ambient-ring {
  position: absolute;
  top: 50%;
  left: 50%;
  z-index: -1;
  width: 760px;
  height: 760px;
  border: 1px solid rgba(148, 163, 184, 0.12);
  border-radius: 50%;
  background:
    conic-gradient(from 90deg, transparent, rgba(125, 211, 252, 0.22), transparent, rgba(167, 243, 208, 0.18), transparent);
  filter: blur(1px);
  transform: translate(-50%, -50%);
  will-change: transform;
}

.fullscreen-visual-layer {
  position: absolute;
  inset: 0;
  z-index: -1;
  overflow: hidden;
  will-change: transform, opacity;
}

.ambient-grid {
  position: absolute;
  inset: 0;
  background:
    linear-gradient(90deg, rgba(255, 255, 255, 0.045) 1px, transparent 1px),
    linear-gradient(180deg, rgba(255, 255, 255, 0.035) 1px, transparent 1px);
  background-size: 72px 72px;
  mask-image: radial-gradient(circle at center, black 0%, transparent 68%);
}

.ambient-aurora {
  position: absolute;
  border-radius: 50%;
  background: radial-gradient(circle, var(--accent-main), transparent 64%);
  filter: blur(42px);
  opacity: 0.18;
  mix-blend-mode: screen;
  will-change: transform, opacity;
}

.aurora-a {
  top: 18%;
  left: 50%;
  width: 620px;
  height: 620px;
  transform: translateX(-50%);
}

.aurora-b {
  right: 7%;
  bottom: 8%;
  width: 360px;
  height: 360px;
  opacity: 0.14;
}

.aurora-c {
  bottom: 10%;
  left: 8%;
  width: 300px;
  height: 300px;
  opacity: 0.12;
}

.ambient-line {
  position: absolute;
  left: 0;
  width: 100%;
  height: 1px;
  background: linear-gradient(90deg, transparent, var(--accent-line), transparent);
  opacity: 0.36;
}

.line-a {
  top: 31%;
  transform: rotate(-13deg);
}

.line-b {
  bottom: 27%;
  transform: rotate(12deg);
}

.fullscreen-copy {
  position: relative;
  z-index: 2;
  min-height: calc(100vh - 72px);
}

.fullscreen-copy-panel {
  position: absolute;
  inset: 0;
  display: grid;
  align-content: center;
  justify-items: center;
  min-height: calc(100vh - 72px);
  gap: 18px;
  text-align: center;
  will-change: transform, opacity;
}

.stage-kicker,
.fullscreen-final p {
  margin: 0;
  color: #7dd3fc;
  font-size: 14px;
  font-weight: 950;
  letter-spacing: 0;
  text-transform: uppercase;
}

.fullscreen-copy-panel h1,
.fullscreen-copy-panel h2 {
  margin: 0;
  color: #ffffff;
  font-size: 88px;
  font-weight: 950;
  line-height: 0.94;
}

.fullscreen-copy-panel strong {
  display: block;
  max-width: 960px;
  color: rgba(248, 250, 252, 0.8);
  font-size: 56px;
  font-weight: 950;
  line-height: 1.04;
}

.fullscreen-copy-panel > span {
  display: block;
  max-width: 860px;
  color: rgba(226, 232, 240, 0.82);
  font-size: 21px;
  font-weight: 650;
  line-height: 1.78;
}

.fullscreen-actions,
.stage-facts {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  align-items: center;
  justify-content: center;
  margin-top: 18px;
}

.stage-primary,
.stage-secondary {
  display: inline-flex;
  min-height: 56px;
  align-items: center;
  justify-content: center;
  gap: 10px;
  border-radius: 999px;
  padding: 0 26px;
  font-size: 17px;
  font-weight: 950;
  transition:
    transform 0.2s ease,
    border-color 0.2s ease,
    background-color 0.2s ease;
}

.stage-primary {
  border: 1px solid rgba(167, 243, 208, 0.58);
  background: linear-gradient(135deg, #f8fafc, #a7f3d0);
  color: #04111f;
  box-shadow: 0 28px 72px rgba(45, 212, 191, 0.2);
}

.stage-secondary {
  border: 1px solid rgba(148, 163, 184, 0.28);
  background: rgba(15, 23, 42, 0.52);
  color: #f8fafc;
}

.stage-primary:hover,
.stage-secondary:hover {
  transform: translateY(-2px);
}

.stage-facts span {
  display: inline-flex;
  align-items: baseline;
  gap: 8px;
  border: 1px solid rgba(148, 163, 184, 0.2);
  border-radius: 999px;
  background: rgba(15, 23, 42, 0.48);
  color: rgba(226, 232, 240, 0.76);
  padding: 10px 16px;
  font-size: 14px;
  font-weight: 850;
}

.stage-facts b {
  color: #ffffff;
  font-size: 24px;
}

.stage-progress {
  position: absolute;
  top: 50%;
  right: 28px;
  z-index: 4;
  display: grid;
  gap: 10px;
  transform: translateY(-50%);
}

.stage-progress span {
  width: 8px;
  height: 34px;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.28);
  transition:
    height 0.2s ease,
    background-color 0.2s ease;
}

.stage-progress span.active {
  height: 62px;
  background: #d6845f;
}

.scroll-cue {
  position: absolute;
  bottom: 24px;
  left: 50%;
  z-index: 4;
  display: grid;
  justify-items: center;
  gap: 8px;
  color: rgba(226, 232, 240, 0.58);
  font-size: 12px;
  font-weight: 900;
  transform: translateX(-50%);
}

.scroll-cue i {
  width: 1px;
  height: 42px;
  background: linear-gradient(180deg, rgba(226, 232, 240, 0.7), transparent);
}

.fullscreen-final {
  display: grid;
  min-height: calc(100vh - 72px);
  align-items: center;
  text-align: center;
  background:
    radial-gradient(circle at 50% 28%, rgba(212, 132, 95, 0.18), transparent 32%),
    linear-gradient(180deg, #050914, #090f1d);
}

.fullscreen-final h2 {
  max-width: 920px;
  margin: 18px auto 0;
  color: #ffffff;
  font-size: 66px;
  font-weight: 950;
  line-height: 1.02;
}

:global(html[data-theme="light"]) .home-fullscreen {
  background: #f7fbff;
  color: #172033;
}

:global(html[data-theme="light"]) .stage-background,
:global(html[data-theme="light"]) .fullscreen-final {
  background:
    radial-gradient(circle at calc(50% + var(--stage-shift, 0%)) 26%, var(--accent-soft), transparent 28%),
    radial-gradient(circle at 52% 72%, rgba(14, 165, 233, 0.14), transparent 30%),
    linear-gradient(180deg, #f8fbff 0%, #edf5f8 100%);
}

:global(html[data-theme="light"]) .stage-vignette {
  background:
    radial-gradient(ellipse at center, transparent 34%, rgba(221, 231, 239, 0.62) 100%),
    linear-gradient(180deg, rgba(248, 251, 255, 0.08), rgba(248, 251, 255, 0.38));
}

:global(html[data-theme="light"]) .fullscreen-copy-panel h1,
:global(html[data-theme="light"]) .fullscreen-copy-panel h2,
:global(html[data-theme="light"]) .fullscreen-final h2 {
  color: #101828;
}

:global(html[data-theme="light"]) .fullscreen-copy-panel strong {
  color: rgba(23, 32, 51, 0.72);
}

:global(html[data-theme="light"]) .fullscreen-copy-panel > span {
  color: rgba(23, 32, 51, 0.72);
}

:global(html[data-theme="light"]) .stage-secondary,
:global(html[data-theme="light"]) .stage-facts span {
  border-color: rgba(15, 23, 42, 0.14);
  background: rgba(255, 255, 255, 0.68);
  color: #172033;
}

:global(html[data-theme="light"]) .stage-facts b {
  color: #101828;
}

@media (max-width: 899px) {
  .fullscreen-pin,
  .fullscreen-copy,
  .fullscreen-copy-panel,
  .fullscreen-final {
    min-height: auto;
  }

  .fullscreen-copy {
    display: grid;
    gap: 0;
  }

  .fullscreen-copy-panel,
  .fullscreen-visual-layer {
    position: relative;
    inset: auto;
  }

  .fullscreen-copy-panel {
    min-height: calc(100vh - 96px);
    border-bottom: 1px solid rgba(148, 163, 184, 0.16);
    padding-top: 48px;
    padding-bottom: 54px;
  }

  .fullscreen-copy-panel h1,
  .fullscreen-copy-panel h2 {
    font-size: 54px;
  }

  .fullscreen-copy-panel strong {
    font-size: 34px;
  }

  .fullscreen-copy-panel > span {
    font-size: 17px;
  }

  .fullscreen-visual-layer,
  .ambient-ring,
  .stage-progress,
  .scroll-cue {
    display: none;
  }

  .fullscreen-final {
    padding: 70px 0;
  }

  .fullscreen-final h2 {
    font-size: 42px;
  }
}

@media (max-width: 560px) {
  .fullscreen-copy-panel h1,
  .fullscreen-copy-panel h2 {
    font-size: 46px;
  }

  .fullscreen-copy-panel strong {
    font-size: 30px;
  }

  .fullscreen-actions,
  .stage-facts {
    width: 100%;
  }

  .stage-primary,
  .stage-secondary,
  .stage-facts span {
    width: 100%;
  }
}

@media (prefers-reduced-motion: reduce) {
  .stage-primary,
  .stage-secondary,
  .stage-progress span {
    transition: none;
  }
}
</style>
