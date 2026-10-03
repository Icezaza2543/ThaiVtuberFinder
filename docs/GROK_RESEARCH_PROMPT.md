# Grok research brief

ใช้กับ Grok CLI ที่รันอยู่ใน root ของ repo นี้ หน้าที่ของ Grok คือ **ค้นหาอย่างเดียว**
ไฟล์ผลลัพธ์จะถูกนำเข้าเป็น seed ภายหลังโดยคนหรือ Claude ตาม AGENTS.md

```bash
grok "อ่าน docs/GROK_RESEARCH_PROMPT.md แล้วทำตามทั้งหมด"
```

---

## บทบาท

คุณช่วยหา VTuber ไทยและ V ประเภทอื่น (VTuber, PNGTuber, VSinger, VArtist) ที่ยังไม่มีในทะเบียน
อ่าน `AGENTS.md` ก่อนเริ่ม แล้วทำตามกฎในนั้นด้วย

## ขอบเขตการแก้ไข repo

- เขียนไฟล์ได้เฉพาะใน `data/grok/` (โฟลเดอร์นี้ถูก gitignore ไว้แล้ว)
- ห้ามแก้ `config/`, `internal/`, `cmd/` และ `docs/`
- ห้าม git commit, push, เปิด PR, deploy หรือรันคำสั่ง `railway`
- ห้ามอ่านหรือพิมพ์ `.env` และ `secrets/`

## Skip set

1. อ่าน `"url"` จาก `config/seeds/*.jsonl` และ `config/easydonate-seed.jsonl` ทุกไฟล์
2. อ่าน `data/grok/*.jsonl` ที่เคยส่งไปแล้ว (ถ้ามี) เพื่อไม่ให้ส่งซ้ำ
   และอ่าน URL ทั้งหมดใน `../ThaiVtuberMaster/web/data/site.json` (ทะเบียนที่เผยแพร่แล้ว อ่านอย่างเดียว)
   รอบที่แล้ว 18 จาก 42 แถวซ้ำกับไฟล์นี้
3. normalize ก่อนเทียบ: ตัด `https://`, `www.`, `/` ท้าย, เปลี่ยน `twitter.com` เป็น `x.com` และเทียบแบบตัวพิมพ์เล็ก
   สำหรับ YouTube ให้เทียบด้วยรหัส `UC…` ด้วย
4. ถ้าอยู่ใน skip set แล้ว ห้ามส่งซ้ำ ทะเบียนจริงใหญ่กว่านี้ ฝั่งเราจะตัดซ้ำอีกรอบ

## ทำแล้ว ไม่ต้องทำซ้ำ

- vtuberthai.com: ดึงครบทั้ง 1,402 โปรไฟล์
- hololist.net/language/thai
- vtuber.chuysan.com: เป็นแหล่งเดียวกับ `kerlos-chuysan-directory` ที่ Finder ดึงทุกรอบอยู่แล้ว
- vtuberthaiinfo archive, Twitch TH, Tipjai, Tipme, Bluesky search (Finder มี adapter แล้ว)
- Echoria part 1–2, Prismx (Mythical Soul set 1–2, Me1odyne, .NXT), โควตเธรด MasshiiMaro

## ขอบเขต

เก็บทุกคนที่เป็นวี ไม่ว่าจะเดบิวต์แล้ว กำลังจะเดบิวต์ หรือเลิกไปแล้ว
VReader และประเภทอื่นที่เป็นวีให้ใช้ `type=other_v` ส่วนบัญชีของค่ายให้เก็บด้วย `type=organization`

## Frontier เรียงตามลำดับความสำคัญ

1. รายชื่อสมาชิกค่ายที่ยังไม่ครบ: 21PM, ManyVProject, 4AM, EverJoyyy, Lumina Project, FvP Project, Sky Rise Project,
   Parabellum Project, DPX, The Solstice (TST), ZYP, Hestia, Kaijieo, VTG13TH, V-LUP, Aegis Unit, LiLiHo Project, Ti19t
   แหล่งที่ใช้ได้: เว็บของค่าย, โพสต์ประกาศรายชื่อสมาชิก, หน้า `/channels` บน YouTube ของค่าย
2. Prismx ยูนิตหลัง .NXT และ Echoria part 3 เป็นต้นไป (ถ้ามี)
3. โควตที่ยังเหลือของเธรดวีอิสระ https://x.com/i/status/1792051626288828574
4. ไทม์ไลน์ของ @DaliyVtuberThai ก่อน ส.ค. 2026
5. รายชื่อแขกของงานอีเวนต์หรือคอลแลบ (V:WORLD ฯลฯ)
6. คำค้นบน X ปี 2022–2023: "วีอิสระ", "เผยโมเดล", "VArtist TH", "#PNGTuberTH", "#VTwitterTH"

## กฎ

- ใช้เฉพาะข้อมูลสาธารณะของตัวครีเอเตอร์ ห้ามเก็บบัญชีผู้ชม แฟนคลับ หรือคนที่มาคอมเมนต์
- Persona ≠ account: ห้ามใส่บัญชีของคนวาด (mama/papa), คนทำ rig, ผู้จัดการ หรือคนตัดต่อมาเป็นตัว V
  handle ที่ปรากฏในเครดิตของ V หลายคนมักเป็นของคนวาดหรือคนทำ rig ถ้าไม่แน่ใจให้ใส่ flag `identity_review`
- ห้ามสรุปเองว่าหลายบัญชีเป็นคนเดียวกัน re-debut หรือโมเดลใหม่ให้นับเป็นอีกรายการ แล้วระบุไว้ใน note
- ห้ามแต่ง URL, handle หรือ ID ขึ้นเอง ทุกแถวต้องมี `source_url` ที่เปิดดูได้จริง
- `url` ต้องเป็นลิงก์ช่องหรือโปรไฟล์ของตัว V เท่านั้น: `x.com/<handle>`, `youtube.com/@<handle>` หรือ `/channel/UC…`,
  `twitch.tv/<login>`, `tiktok.com/@<handle>`, `bsky.app/profile/<handle>`
  - ห้ามใช้ลิงก์หน้าเว็บไดเรกทอรีหรือลิงก์คลิป (`/watch?`, `/live/`, `/shorts/`) ใส่ไว้ใน `source_url` ได้
- บัญชีค่ายหรือกลุ่ม ให้ใส่ `type=organization`
- เคารพ robots.txt และ rate limit ถ้าเว็บบล็อกหรือขึ้น checkpoint ให้หยุดเว็บนั้นแล้วบันทึกไว้ใน CHECKPOINT ห้ามพยายามหลบ

## ผลลัพธ์

เขียนลง `data/grok/` ทั้งหมด:

1. `data/grok/grok-research-<YYYY-MM-DD>-<n>.jsonl` บรรทัดละ 1 URL
   (ถ้า V คนเดียวมีหลายแพลตฟอร์ม ให้แยกบรรทัดโดยใช้ `name` เดียวกัน)

   ```json
   {"url":"https://x.com/handle","name":"ชื่อที่แสดง","source_url":"https://หลักฐาน","description":"Grok research <YYYY-MM-DD>; type=vtuber|pngtuber|vsinger|vartist|organization|other_v; agency=<ค่าย หรือ Independent>; confidence=high|medium|low; flags=<identity_review;subtype_unclear;thai_signal_unclear หรือเว้นว่าง>; <หมายเหตุสั้น ๆ>"}
   ```

2. `data/grok/CHECKPOINT-<YYYY-MM-DD>-<n>.md` ระบุ:
   - จำนวนแถว แยกตาม confidence และตามแพลตฟอร์ม
   - frontier ที่ทำแล้ว และทำไปถึงตรงไหน
   - frontier ที่เหลือ และข้อจำกัดที่เจอ

ก่อนจบ ให้ตรวจไฟล์ JSONL ว่าทุกบรรทัด parse ได้ ไม่มีแถวซ้ำกันเอง และไม่มีแถวที่อยู่ใน skip set
