// Cuts the two recordings of record.spec.ts into one 1920×1080 film by the marks of timeline.json:
// the client up to the moment the application waits, the analyst at work, both side by side while the decision
// reaches the client live, then the client to the end. Needs ffmpeg.
//   node demo-video/edit.mjs [out/matritsa-demo.mp4]
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const dir = path.join(path.dirname(fileURLToPath(import.meta.url)), 'out')
const target = path.resolve(process.argv[2] ?? path.join(dir, 'matritsa-demo.mp4'))
const { clientStart, staffStart, marks } = JSON.parse(fs.readFileSync(path.join(dir, 'timeline.json'), 'utf8'))
const at = (name) => {
  const m = marks.find((x) => x.name === name)
  if (!m) throw new Error(`no mark ${name}`)
  return m.at
}
const c = (name) => ((at(name) - clientStart) / 1000).toFixed(3) // seconds into the client recording
const s = (name) => ((at(name) - staffStart) / 1000).toFixed(3) // seconds into the analyst recording

const W = 1920
const H = 1080
const BG = '0x0f0f14'
const full = (input, from, to, label) =>
  `[${input}:v]trim=start=${from}:end=${to},setpts=PTS-STARTPTS,fps=30,scale=-2:${H},pad=${W}:${H}:(ow-iw)/2:0:color=${BG},setsar=1[${label}]`

// Side by side: the decision panel of the analyst on the left, the main column of the client on the right. Each is a
// 889×1000 part of its 1600×1000 window shown at 960×1080, so the text stays as readable as in the full scenes.
const part = (input, x, from, to, label) =>
  `[${input}:v]trim=start=${from}:end=${to},setpts=PTS-STARTPTS,fps=30,crop=889:1000:${x}:0,scale=960:1080,setsar=1[${label}]`
const split = [
  part(1, 600, s('split-begin'), s('split-end'), 'sl'),
  part(0, 100, c('split-begin'), c('split-end'), 'sr'),
  `[sl][sr]hstack=inputs=2,drawbox=x=958:y=0:w=4:h=${H}:color=${BG}:t=fill[split]`,
]

const graph = [
  full(0, 0, c('client-waits'), 'a'),
  full(1, s('staff-begin'), s('split-begin'), 'b'),
  ...split,
  full(0, c('split-end'), c('client-end'), 'd'),
  '[a][b][split][d]concat=n=4:v=1:a=0[out]',
].join(';')

execFileSync('ffmpeg', [
  '-y', '-loglevel', 'error',
  '-i', path.join(dir, 'client.webm'),
  '-i', path.join(dir, 'staff.webm'),
  '-filter_complex', graph, '-map', '[out]',
  '-c:v', 'libx264', '-preset', 'slow', '-crf', '18', '-pix_fmt', 'yuv420p', '-movflags', '+faststart',
  target,
], { stdio: 'inherit' })
console.log(`film: ${target}`)
